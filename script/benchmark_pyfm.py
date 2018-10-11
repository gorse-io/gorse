# This script runs a 5-Fold CV for all the algorithms (default parameters) on
# the movielens datasets, and reports average RMSE, MAE, and total computation
# time. Usage:
#
#   python3 benchmark_pyfm.py [regression|classification] [--content]
#
# For example:
#
#   python3 benchmark_pyfm.py regression --content
#
# The command runs benchmark with regression and content features.

import math
import time
import random
import sys

import numpy as np
import pandas as pd
from pyfm import pylibfm
from sklearn.feature_extraction import DictVectorizer
from sklearn.metrics import mean_squared_error, log_loss
from surprise import Dataset
from surprise import builtin_datasets


def root_mean_squared_error(*args):
    return math.sqrt(mean_squared_error(*args))


# Read in data
def load_ml_100k(file_rating="ml-100k", content=False):
    # Download data
    Dataset.load_builtin("ml-100k")
    # Load data
    file_rating = builtin_datasets.get_dataset_dir() + 'ml-100k/ml-100k/' + file_rating
    df_rating = pd.read_csv(file_rating, sep='\t', names=['user_id', 'item_id', 'rating', 'timestamp'],
                            dtype={'user_id': str, 'item_id': str, 'rating': np.float})
    df_x = df_rating[['user_id', 'item_id']]
    # Load content
    if content:
        file_movie = builtin_datasets.get_dataset_dir() + 'ml-100k/ml-100k/u.item'
        file_user = builtin_datasets.get_dataset_dir() + 'ml-100k/ml-100k/u.user'
        df_movie = pd.read_csv(file_movie, sep='|', encoding="ISO-8859-1",
                               names=['item_id', 'movie_title', 'release_date', 'video_release_date',
                                      'IMDb_URL', 'unknown', 'Action', 'Adventure', 'Animation',
                                      'Children\'s', 'Comedy', 'Crime', 'Documentary', 'Drama', 'Fantasy',
                                      'Film-Noir', 'Horror', 'Musical', 'Mystery', 'Romance', 'Sci-Fi',
                                      'Thriller', 'War', 'Western'],
                               dtype={'item_id': str})
        df_user = pd.read_csv(file_user, sep='|', names=['user_id', 'age', 'gender', 'occupation', 'zip_code'],
                              dtype={'user_id': str})
        df_user['gender'] = (df_user['gender'] == 'M').astype(np.float)
        df_x = pd.merge(df_x, df_user[['user_id', 'gender']], how='left')
        df_x = pd.merge(df_x, df_movie[['item_id', 'unknown', 'Action', 'Adventure', 'Animation',
                                        'Children\'s', 'Comedy', 'Crime', 'Documentary', 'Drama', 'Fantasy',
                                        'Film-Noir', 'Horror', 'Musical', 'Mystery', 'Romance', 'Sci-Fi',
                                        'Thriller', 'War', 'Western']], how='left')
    return df_x.to_dict(orient='records'), np.array(df_rating['rating'])


def benchmark(task='regression', content=False):
    losses = []
    total_time = 0
    for k in range(5):
        print('== Fold %d' % (k+1))
        # set RNG
        np.random.seed(0)
        random.seed(0)
        # Load data
        (train_data, y_train) = load_ml_100k("u%d.base" % (k+1), content)
        (test_data, y_test) = load_ml_100k("u%d.test" % (k+1), content)
        if task == "classification":
            y_test = np.greater(y_test, 3)
        # Transform to matrix
        v = DictVectorizer()
        x_train = v.fit_transform(train_data)
        x_test = v.transform(test_data)
        # Build and train a Factorization Machine
        fm = pylibfm.FM(num_iter=20, verbose=True, task=task,
                        initial_learning_rate=0.005,
                        learning_rate_schedule="constant", seed=0)
        start = time.time()
        fm.fit(x_train, y_train)
        used = time.time() - start
        total_time += used
        # Evaluate
        predictions = fm.predict(x_test)
        if task == "regression":
            losses.append(root_mean_squared_error(y_test, predictions))
            print("FM RMSE: %.4f" % losses[-1])
        elif task == "classification":
            losses.append(log_loss(y_test, predictions))
            print("FM log loss: %.4f" % losses[-1])
        print("Time used: %.4fs" % used)
    print('== Summary')
    print('Mean RMSE: %.4f' % np.mean(losses))
    print('Total time: %.4fs' % total_time)


if __name__ == "__main__":
    task = 'regression'
    content = False
    if len(sys.argv) > 1:
        task = sys.argv[1]
    if len(sys.argv) > 2:
        content = sys.argv[2] == "--content"
    benchmark(task, content)
