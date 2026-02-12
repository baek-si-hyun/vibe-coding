"""
삼성전자 주가 예측 모델 (LSTM + CNN)
주체별 매매 동향 데이터로 향후 주가 수익률을 추정한다.
"""
import os
from pathlib import Path
from tensorflow.keras.layers import Dense, Input, LSTM, Average, Conv2D, MaxPooling2D, Flatten
from tensorflow.keras.models import Model
from tensorflow.keras.optimizers import Adam
import numpy as np
import pandas as pd
import matplotlib
matplotlib.use('Agg')  # GUI 없이 사용
import matplotlib.pyplot as plt


def build_train_data(data, t_step, n_jump=1):
    """
    2차원 배열의 feature 데이터로 LSTM 학습 데이터를 만든다.
    
    Args:
        data: 2D 배열 (n_data, n_feat)
        t_step: 시계열 길이
        n_jump: 샘플링 간격
    
    Returns:
        x_data: (n_samples, t_step, n_feat)
        y_target: (n_samples, n_feat)
    """
    n_data = data.shape[0]
    n_feat = data.shape[1]

    m = np.arange(0, n_data - t_step, n_jump)
    x = [data[i:(i+t_step), :] for i in m]
    y = [data[i, :] for i in (m + t_step)]

    x_data = np.reshape(np.array(x), (len(m), t_step, n_feat))
    y_target = np.reshape(np.array(y), (len(m), n_feat))
    
    return x_data, y_target


def load_and_prepare_data(data_path):
    """
    주가 데이터를 로드하고 전처리한다.
    
    Args:
        data_path: CSV 파일 경로
    
    Returns:
        df: 전처리된 데이터프레임
        last_price: 마지막 종가
        rtn_mean: 등락율 평균 (복원용)
        rtn_std: 등락율 표준편차 (복원용)
    """
    data = pd.read_csv(data_path, encoding='euc-kr')
    last_price = list(data['종가'])[-1]
    
    df = data.dropna()
    df = df.drop(['날짜', '종가', '전일비', '개인누적', '기관누적', '외국인누적', 
                  '금투누적', '투신누적', '연기금누적', '국가지자체'], axis=1)
    
    # feature 표준화
    rtn_mean = df['등락율'].mean()
    rtn_std = df['등락율'].std()
    df_normalized = (df - df.mean()) / df.std()
    
    return df_normalized, last_price, rtn_mean, rtn_std


def create_model(t_step, n_feat, n_hidden=128):
    """
    LSTM + CNN 하이브리드 모델을 생성한다.
    
    Args:
        t_step: 시계열 길이
        n_feat: feature 개수
        n_hidden: hidden layer 크기
    
    Returns:
        model: 컴파일된 모델
    """
    # LSTM 모델
x_lstm_input = Input(batch_shape=(None, t_step, n_feat))
x_lstm = LSTM(n_hidden, return_sequences=True)(x_lstm_input)
x_lstm = LSTM(n_hidden, dropout=0.2)(x_lstm)

    # CNN 모델
    x_cnn_input = Input(batch_shape=(None, t_step, n_feat, 1))
    x_conv = Conv2D(filters=10, kernel_size=(20, 5), strides=1, padding='same', activation='relu')(x_cnn_input)
    x_pool = MaxPooling2D(pool_size=(10, 5), strides=1, padding='same')(x_conv)
x_flat = Flatten()(x_pool)
x_cnn = Dense(n_hidden)(x_flat)

    # 두 네트워크를 합친다
x_avg = Average()([x_lstm, x_cnn])
y_output = Dense(n_feat)(x_avg)

model = Model([x_lstm_input, x_cnn_input], y_output)
model.compile(loss='mse', optimizer=Adam(learning_rate=0.001))
    
    return model


def train_model(model, x_train, y_train, epochs=50, batch_size=32):
    """
    모델을 학습시킨다.
    
    Args:
        model: 학습할 모델
        x_train: 학습 데이터
        y_train: 타겟 데이터
        epochs: 학습 에포크 수
        batch_size: 배치 크기
    
    Returns:
        history: 학습 히스토리
    """
    t_step = x_train.shape[1]
    n_feat = x_train.shape[2]
    
    history = model.fit(
        [x_train, x_train.reshape(-1, t_step, n_feat, 1)],
        y_train,
        epochs=epochs,
        batch_size=batch_size,
        shuffle=True,
        verbose=1
    )
    
    return history


def predict_next_day(model, df, t_step, rtn_mean, rtn_std, last_price):
    """
    내일의 수익률과 주가를 예측한다.
    
    Args:
        model: 학습된 모델
        df: 전처리된 데이터프레임
        t_step: 시계열 길이
        rtn_mean: 등락율 평균
        rtn_std: 등락율 표준편차
        last_price: 마지막 종가
    
    Returns:
        predicted_return: 예측된 수익률
        predicted_price: 예측된 주가
    """
    n_feat = df.shape[1]
    px_lstm = np.array(df.tail(t_step)).reshape(1, t_step, n_feat)
    px_cnn = px_lstm.reshape(1, t_step, n_feat, 1)
    
    y_pred = model.predict([px_lstm, px_cnn], verbose=0)[0][0]
    y_rtn = y_pred * rtn_std + rtn_mean
    
    predicted_price = last_price * (1 + y_rtn)
    
    return y_rtn, predicted_price


def main(data_path=None):
    """
    메인 실행 함수
    
    Args:
        data_path: 데이터 파일 경로 (None이면 현재 디렉토리에서 찾음)
    """
    # 데이터 경로 설정
    if data_path is None:
        script_dir = Path(__file__).parent
        data_path = script_dir / 'data' / '삼성전자.csv'
    
    if not os.path.exists(data_path):
        print(f"데이터 파일을 찾을 수 없습니다: {data_path}")
        print("사용법: python stock_predictor.py <데이터_파일_경로>")
        return
    
    print(f"데이터 로드 중: {data_path}")
    df, last_price, rtn_mean, rtn_std = load_and_prepare_data(data_path)
    
    t_step = 20
    print(f"학습 데이터 생성 중...")
    x_train, y_train = build_train_data(np.array(df), t_step, n_jump=1)
    print(f"학습 데이터 shape: {x_train.shape}, {y_train.shape}")
    
    n_feat = x_train.shape[2]
    n_hidden = 128
    
    print("모델 생성 중...")
    model = create_model(t_step, n_feat, n_hidden)
model.summary()

    print("모델 학습 중...")
    history = train_model(model, x_train, y_train, epochs=50, batch_size=32)

# Loss history를 그린다
plt.figure(figsize=(8, 3))
    plt.plot(history.history['loss'], color='red')
plt.title("Loss History")
plt.xlabel("epoch")
plt.ylabel("loss")
    plt.savefig('loss_history.png')
    print("Loss history 그래프 저장: loss_history.png")

    # 내일의 수익률과 주가를 예측한다
    print("\n예측 중...")
    predicted_return, predicted_price = predict_next_day(
        model, df, t_step, rtn_mean, rtn_std, last_price
    )
    
    if predicted_return > 0:
        print(f"내일은 {predicted_return * 100:.2f}% 상승할 것으로 예측됩니다.")
else:
        print(f"내일은 {predicted_return * 100:.2f}% 하락할 것으로 예측됩니다.")
    print(f"예상 주가 = {predicted_price:.0f}")
    
    return model, predicted_return, predicted_price


if __name__ == "__main__":
    import sys
    
    data_path = sys.argv[1] if len(sys.argv) > 1 else None
    main(data_path)
