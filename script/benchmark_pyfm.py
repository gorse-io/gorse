import math
import random

import numpy as np
import pandas as pd
from pyfm import pylibfm
from sklearn.feature_extraction import DictVectorizer
from sklearn.metrics import mean_squared_error
from surprise import Dataset
from surprise import builtin_datasets


def root_mean_squared_error(*args):
    return math.sqrt(mean_squared_error(*args))


# Read in data
def load_ml_100k(file_rating="ml-100k"):
    # Download data
    Dataset.load_builtin("ml-100k")
    # Load data
    file_rating = builtin_datasets.get_dataset_dir() + 'ml-100k/ml-100k/' + file_rating
    file_movie = builtin_datasets.get_dataset_dir() + 'ml-100k/ml-100k/u.item'
    file_user = builtin_datasets.get_dataset_dir() + 'ml-100k/ml-100k/u.user'
    df_rating = pd.read_csv(file_rating, sep='\t', names=['user_id', 'item_id', 'rating', 'timestamp'],
                            dtype={'user_id': str, 'item_id': str, 'rating': np.float})
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
    df_x = pd.merge(df_rating[['user_id', 'item_id']], df_user[['user_id']], how='left')
    # df_x = pd.merge(df_x, df_movie[['item_id', 'unknown', 'Action', 'Adventure', 'Animation',
    #                                 'Children\'s', 'Comedy', 'Crime', 'Documentary', 'Drama', 'Fantasy',
    #                                 'Film-Noir', 'Horror', 'Musical', 'Mystery', 'Romance', 'Sci-Fi',
    #                                 'Thriller', 'War', 'Western']], how='left')
    return df_x.to_dict(orient='records'), np.array(df_rating['rating'])


def main():
    rmses = []
    for k in range(5):
        print('== Fold %d' % (k+1))
        # set RNG
        np.random.seed(0)
        random.seed(0)
        # Load data
        (train_data, y_train) = load_ml_100k("u%d.base" % (k+1))
        (test_data, y_test) = load_ml_100k("u%d.test" % (k+1))
        # Transform to matrix
        v = DictVectorizer()
        x_train = v.fit_transform(train_data)
        x_test = v.transform(test_data)
        # Build and train a Factorization Machine
        fm = pylibfm.FM(num_iter=20, verbose=True, task="regression",
                        initial_learning_rate=0.005,
                        learning_rate_schedule="constant", seed=0)
        fm.fit(x_train, y_train)
        # Evaluate
        predictions = fm.predict(x_test)
        rmses.append(root_mean_squared_error(y_test, predictions))
        print("FM RMSE: %.4f" % rmses[-1])
    print('== Summary')
    print('Mean RMSE: %.4f' % np.mean(rmses))


if __name__ == "__main__":
    main()
