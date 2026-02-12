"""
LSTM 모델 유틸리티 함수
"""
import numpy as np


def build_train_data_many_to_one(data, t_step, n_jump=1):
    """
    Many-to-One 방식: 여러 시점의 데이터로 다음 한 시점을 예측
    
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


def build_train_data_many_to_many(data, t_step, n_jump=1):
    """
    Many-to-Many 방식: 여러 시점의 데이터로 다음 여러 시점을 예측
    
    Args:
        data: 2D 배열 (n_data, n_feat)
        t_step: 시계열 길이
        n_jump: 샘플링 간격
    
    Returns:
        x_data: (n_samples, t_step, n_feat)
        y_target: (n_samples, t_step, n_feat)
    """
    n_data = data.shape[0]
    n_feat = data.shape[1]

    m = np.arange(0, n_data - t_step, n_jump)
    x = [data[i:(i+t_step)] for i in m]
    y = [data[(i+1):(i+1+t_step), :] for i in m]

    x_data = np.reshape(np.array(x), (len(m), t_step, n_feat))
    y_target = np.reshape(np.array(y), (len(m), t_step, n_feat))

    return x_data, y_target
