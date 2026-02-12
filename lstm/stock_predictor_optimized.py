import os
from pathlib import Path
from tensorflow.keras.layers import (
    Dense, Input, LSTM, Bidirectional, Average, Conv2D, MaxPooling2D, 
    Flatten, BatchNormalization, Dropout, Concatenate
)
from tensorflow.keras.models import Model
from tensorflow.keras.optimizers import Adam
from tensorflow.keras.callbacks import EarlyStopping, ReduceLROnPlateau, ModelCheckpoint
from tensorflow.keras.regularizers import l1_l2
import numpy as np
import pandas as pd
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from sklearn.model_selection import train_test_split
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score


def build_train_data(data, t_step, n_jump=1):
    n_data = data.shape[0]
    n_feat = data.shape[1]

    m = np.arange(0, n_data - t_step, n_jump)
    x = [data[i:(i+t_step), :] for i in m]
    y = [data[i, :] for i in (m + t_step)]

    x_data = np.reshape(np.array(x), (len(m), t_step, n_feat))
    y_target = np.reshape(np.array(y), (len(m), n_feat))
    
    return x_data, y_target


def load_and_prepare_data(data_path, test_size=0.2):
    data = pd.read_csv(data_path, encoding='euc-kr')
    last_price = list(data['종가'])[-1]
    
    df = data.dropna()
    df = df.drop(['날짜', '종가', '전일비', '개인누적', '기관누적', '외국인누적', 
                  '금투누적', '투신누적', '연기금누적', '국가지자체'], axis=1)
    
    feature_stats = {}
    for col in df.columns:
        feature_stats[col] = {
            'mean': df[col].mean(),
            'std': df[col].std()
        }
    
    rtn_mean = df['등락율'].mean()
    rtn_std = df['등락율'].std()
    
    df_normalized = (df - df.mean()) / df.std()
    
    return df_normalized, last_price, rtn_mean, rtn_std, feature_stats


def create_optimized_model(t_step, n_feat, n_hidden=256, dropout_rate=0.3):
    x_lstm_input = Input(shape=(t_step, n_feat), name='lstm_input')
    
    lstm1 = Bidirectional(
        LSTM(n_hidden, return_sequences=True, 
             kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4)),
        name='bidirectional_lstm1'
    )(x_lstm_input)
    lstm1 = BatchNormalization()(lstm1)
    lstm1 = Dropout(dropout_rate)(lstm1)
    
    lstm2 = Bidirectional(
        LSTM(n_hidden // 2, return_sequences=False,
             kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4)),
        name='bidirectional_lstm2'
    )(lstm1)
    lstm2 = BatchNormalization()(lstm2)
    lstm2 = Dropout(dropout_rate)(lstm2)
    
    x_cnn_input = Input(shape=(t_step, n_feat, 1), name='cnn_input')
    
    conv1 = Conv2D(
        filters=32, 
        kernel_size=(5, 3), 
        strides=1, 
        padding='same', 
        activation='relu',
        kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4),
        name='conv2d_1'
    )(x_cnn_input)
    conv1 = BatchNormalization()(conv1)
    conv1 = MaxPooling2D(pool_size=(2, 2), name='maxpool_1')(conv1)
    
    conv2 = Conv2D(
        filters=64,
        kernel_size=(3, 3),
        strides=1,
        padding='same',
        activation='relu',
        kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4),
        name='conv2d_2'
    )(conv1)
    conv2 = BatchNormalization()(conv2)
    conv2 = MaxPooling2D(pool_size=(2, 2), name='maxpool_2')(conv2)
    
    x_flat = Flatten(name='flatten')(conv2)
    
    cnn_dense1 = Dense(n_hidden, activation='relu', 
                      kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4),
                      name='cnn_dense1')(x_flat)
    cnn_dense1 = BatchNormalization()(cnn_dense1)
    cnn_dense1 = Dropout(dropout_rate)(cnn_dense1)
    
    cnn_dense2 = Dense(n_hidden // 2, activation='relu',
                      kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4),
                      name='cnn_dense2')(cnn_dense1)
    cnn_dense2 = BatchNormalization()(cnn_dense2)
    cnn_dense2 = Dropout(dropout_rate)(cnn_dense2)
    
    combined = Concatenate(name='concatenate')([lstm2, cnn_dense2])
    combined = BatchNormalization()(combined)
    combined = Dropout(dropout_rate)(combined)
    
    output_dense = Dense(n_hidden, activation='relu',
                        kernel_regularizer=l1_l2(l1=1e-5, l2=1e-4),
                        name='output_dense')(combined)
    output_dense = BatchNormalization()(output_dense)
    output_dense = Dropout(dropout_rate * 0.5)(output_dense)
    
    y_output = Dense(n_feat, name='final_output')(output_dense)
    
    model = Model([x_lstm_input, x_cnn_input], y_output)
    
    optimizer = Adam(learning_rate=0.001, beta_1=0.9, beta_2=0.999, epsilon=1e-8)
    
    model.compile(
        loss='huber',
        optimizer=optimizer,
        metrics=['mae', 'mse']
    )
    
    return model


def train_optimized_model(model, x_train, y_train, x_val, y_val, 
                         epochs=100, batch_size=64, model_save_path='best_model.h5'):
    callbacks = [
        EarlyStopping(
            monitor='val_loss',
            patience=15,
            restore_best_weights=True,
            verbose=1,
            min_delta=1e-6
        ),
        ReduceLROnPlateau(
            monitor='val_loss',
            factor=0.5,
            patience=5,
            min_lr=1e-7,
            verbose=1
        ),
        ModelCheckpoint(
            model_save_path,
            monitor='val_loss',
            save_best_only=True,
            verbose=1
        )
    ]
    
    t_step = x_train.shape[1]
    n_feat = x_train.shape[2]
    
    history = model.fit(
        [x_train, x_train.reshape(-1, t_step, n_feat, 1)],
        y_train,
        validation_data=(
            [x_val, x_val.reshape(-1, t_step, n_feat, 1)],
            y_val
        ),
        epochs=epochs,
        batch_size=batch_size,
        shuffle=True,
        callbacks=callbacks,
        verbose=1
    )
    
    return history


def evaluate_model(model, x_test, y_test, t_step, n_feat):
    y_pred = model.predict(
        [x_test, x_test.reshape(-1, t_step, n_feat, 1)],
        verbose=0
    )
    
    y_test_rtn = y_test[:, 0]
    y_pred_rtn = y_pred[:, 0]
    
    mae = mean_absolute_error(y_test_rtn, y_pred_rtn)
    mse = mean_squared_error(y_test_rtn, y_pred_rtn)
    rmse = np.sqrt(mse)
    r2 = r2_score(y_test_rtn, y_pred_rtn)
    
    return {
        'mae': mae,
        'mse': mse,
        'rmse': rmse,
        'r2': r2,
        'y_test': y_test_rtn,
        'y_pred': y_pred_rtn
    }


def predict_next_day(model, df, t_step, rtn_mean, rtn_std, last_price):
    n_feat = df.shape[1]
    px_lstm = np.array(df.tail(t_step)).reshape(1, t_step, n_feat)
    px_cnn = px_lstm.reshape(1, t_step, n_feat, 1)
    
    y_pred = model.predict([px_lstm, px_cnn], verbose=0)[0]
    
    y_rtn = y_pred[0] * rtn_std + rtn_mean
    
    predicted_price = last_price * (1 + y_rtn)
    
    return y_rtn, predicted_price


def plot_training_history(history, save_path='training_history.png'):
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    
    axes[0, 0].plot(history.history['loss'], label='Train Loss', color='blue')
    axes[0, 0].plot(history.history['val_loss'], label='Val Loss', color='red')
    axes[0, 0].set_title('Model Loss')
    axes[0, 0].set_xlabel('Epoch')
    axes[0, 0].set_ylabel('Loss')
    axes[0, 0].legend()
    axes[0, 0].grid(True)
    
    axes[0, 1].plot(history.history['mae'], label='Train MAE', color='blue')
    axes[0, 1].plot(history.history['val_mae'], label='Val MAE', color='red')
    axes[0, 1].set_title('Mean Absolute Error')
    axes[0, 1].set_xlabel('Epoch')
    axes[0, 1].set_ylabel('MAE')
    axes[0, 1].legend()
    axes[0, 1].grid(True)
    
    axes[1, 0].plot(history.history['mse'], label='Train MSE', color='blue')
    axes[1, 0].plot(history.history['val_mse'], label='Val MSE', color='red')
    axes[1, 0].set_title('Mean Squared Error')
    axes[1, 0].set_xlabel('Epoch')
    axes[1, 0].set_ylabel('MSE')
    axes[1, 0].legend()
    axes[1, 0].grid(True)
    
    if 'lr' in history.history:
        axes[1, 1].plot(history.history['lr'], label='Learning Rate', color='green')
        axes[1, 1].set_title('Learning Rate')
        axes[1, 1].set_xlabel('Epoch')
        axes[1, 1].set_ylabel('LR')
        axes[1, 1].set_yscale('log')
        axes[1, 1].legend()
        axes[1, 1].grid(True)
    
    plt.tight_layout()
    plt.savefig(save_path, dpi=300, bbox_inches='tight')
    print(f"학습 히스토리 그래프 저장: {save_path}")


def main(data_path=None, t_step=30, n_hidden=256, epochs=100, batch_size=64):
    if data_path is None:
        script_dir = Path(__file__).parent
        data_path = script_dir / 'data' / '삼성전자.csv'
    
    if not os.path.exists(data_path):
        print(f"데이터 파일을 찾을 수 없습니다: {data_path}")
        print("사용법: python stock_predictor_optimized.py <데이터_파일_경로>")
        return
    
    print("=" * 60)
    print("최적화된 주가 예측 모델")
    print("=" * 60)
    
    print(f"\n데이터 로드 중: {data_path}")
    df, last_price, rtn_mean, rtn_std, feature_stats = load_and_prepare_data(data_path)
    print(f"데이터 shape: {df.shape}")
    
    print(f"\n학습 데이터 생성 중... (t_step={t_step})")
    x_data, y_data = build_train_data(np.array(df), t_step, n_jump=1)
    print(f"학습 데이터 shape: {x_data.shape}, {y_data.shape}")
    
    x_train, x_temp, y_train, y_temp = train_test_split(
        x_data, y_data, test_size=0.3, random_state=42, shuffle=False
    )
    x_val, x_test, y_val, y_test = train_test_split(
        x_temp, y_temp, test_size=0.5, random_state=42, shuffle=False
    )
    
    print(f"\n데이터 분할:")
    print(f"  Train: {x_train.shape[0]} samples")
    print(f"  Validation: {x_val.shape[0]} samples")
    print(f"  Test: {x_test.shape[0]} samples")
    
    n_feat = x_train.shape[2]
    
    print(f"\n최적화된 모델 생성 중...")
    print(f"  Hidden units: {n_hidden}")
    print(f"  Dropout rate: 0.3")
    model = create_optimized_model(t_step, n_feat, n_hidden=n_hidden)
    model.summary()
    
    model_save_path = 'best_model_optimized.h5'
    print(f"\n모델 학습 중...")
    print(f"  Max epochs: {epochs}")
    print(f"  Batch size: {batch_size}")
    history = train_optimized_model(
        model, x_train, y_train, x_val, y_val,
        epochs=epochs, batch_size=batch_size,
        model_save_path=model_save_path
    )
    
    plot_training_history(history, 'training_history_optimized.png')
    
    print(f"\n모델 평가 중...")
    eval_results = evaluate_model(model, x_test, y_test, t_step, n_feat)
    print(f"  Test MAE: {eval_results['mae']:.6f}")
    print(f"  Test RMSE: {eval_results['rmse']:.6f}")
    print(f"  Test R²: {eval_results['r2']:.4f}")
    
    print(f"\n예측 중...")
    predicted_return, predicted_price = predict_next_day(
        model, df, t_step, rtn_mean, rtn_std, last_price
    )
    
    print(f"\n{'='*60}")
    print("예측 결과")
    print(f"{'='*60}")
    if predicted_return > 0:
        print(f"내일은 {predicted_return * 100:.2f}% 상승할 것으로 예측됩니다.")
    else:
        print(f"내일은 {predicted_return * 100:.2f}% 하락할 것으로 예측됩니다.")
    print(f"현재 주가: {last_price:,.0f}원")
    print(f"예상 주가: {predicted_price:,.0f}원")
    print(f"예상 변동액: {predicted_price - last_price:+,.0f}원")
    print(f"{'='*60}")
    
    return model, history, eval_results, predicted_return, predicted_price


if __name__ == "__main__":
    import sys
    
    data_path = sys.argv[1] if len(sys.argv) > 1 else None
    main(
        data_path=data_path,
        t_step=30,
        n_hidden=256,
        epochs=100,
        batch_size=64
    )
