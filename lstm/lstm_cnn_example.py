import pandas as pd
import numpy as np
from utils import build_train_data_many_to_one, build_train_data_many_to_many


def example_many_to_one():
    print("=" * 50)
    print("Many-to-One 방식 예제")
    print("=" * 50)
    
    df = pd.DataFrame({
        'f1': np.arange(50),
        'f2': np.arange(0.0, 5, 0.1)
    })
    
    print(f"원본 데이터 shape: {df.shape}")
    print(df.head())
    
    x_train, y_train = build_train_data_many_to_one(
        np.array(df),
        t_step=3,
        n_jump=2
    )
    
    print(f"\n학습 데이터 shape:")
    print(f"x_train: {x_train.shape}")
    print(f"y_train: {y_train.shape}")
    print(f"\nx_train[0]:\n{x_train[0]}")
    print(f"y_train[0]:\n{y_train[0]}")


def example_many_to_many():
    print("\n" + "=" * 50)
    print("Many-to-Many 방식 예제")
    print("=" * 50)
    
    df = pd.DataFrame({
        'f1': np.arange(50),
        'f2': np.arange(0.0, 5, 0.1)
    })
    
    x_train, y_train = build_train_data_many_to_many(
        np.array(df),
        t_step=3,
        n_jump=2
    )
    
    print(f"학습 데이터 shape:")
    print(f"x_train: {x_train.shape}")
    print(f"y_train: {y_train.shape}")
    print(f"\nx_train[0]:\n{x_train[0]}")
    print(f"y_train[0]:\n{y_train[0]}")


if __name__ == "__main__":
    example_many_to_one()
    example_many_to_many()
