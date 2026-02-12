import numpy as np


def build_train_data_many_to_one(data, t_step, n_jump=1):
    n_data = data.shape[0]
    n_feat = data.shape[1]

    m = np.arange(0, n_data - t_step, n_jump)
    x = [data[i:(i+t_step), :] for i in m]
    y = [data[i, :] for i in (m + t_step)]

    x_data = np.reshape(np.array(x), (len(m), t_step, n_feat))
    y_target = np.reshape(np.array(y), (len(m), n_feat))
    
    return x_data, y_target


def build_train_data_many_to_many(data, t_step, n_jump=1):
    n_data = data.shape[0]
    n_feat = data.shape[1]

    m = np.arange(0, n_data - t_step, n_jump)
    x = [data[i:(i+t_step)] for i in m]
    y = [data[(i+1):(i+1+t_step), :] for i in m]

    x_data = np.reshape(np.array(x), (len(m), t_step, n_feat))
    y_target = np.reshape(np.array(y), (len(m), t_step, n_feat))

    return x_data, y_target
