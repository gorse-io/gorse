=============
Configuration
=============

Fields
======

``server``
----------

Configurations for service host and port.

+-------+--------+-------------+-----------+
| Field | Type   | Description | Default   |
+=======+========+=============+===========+
| host  | string | server host | 127.0.0.1 |
+-------+--------+-------------+-----------+
| port  | int    | server port | 8080      |
+-------+--------+-------------+-----------+

``database``
------------

Configurations for database location.

+-------+--------+------------------------+-------------------+
| Field | Type   | Description            | Default           |
+=======+========+=========+==============+===================+
| file  | string | database file location | ~/.gorse/gorse.db |
+-------+--------+------------------------+-------------------+

``recommend``
-------------

Configurations for recommendation.

+------------------+--------+----------------------------------------------------------------------------------+----------+
| Field            | Type   | Description                                                                      | Default  |
+==================+========+==================================================================================+==========+
| model            | string | recommendation model                                                             | bpr      |
+------------------+--------+----------------------------------------------------------------------------------+----------+
| cache_size       | int    | the number of cached recommendations                                             | 100      |
+------------------+--------+----------------------------------------------------------------------------------+----------+
| update_threshold | int    | update model when more than ``n`` ratings are added                              | 10       |
+------------------+--------+----------------------------------------------------------------------------------+----------+
| check_period     | int    | check for update every one minute                                                | 1        |
+------------------+--------+----------------------------------------------------------------------------------+----------+
| similarity       | string | similarity metric for neighbors (``pearson``, ``cosine``, ``msd``, ``implicit``) | implicit |
+------------------+--------+----------------------------------------------------------------------------------+----------+
| fit_jobs         | int    | concurrent jobs for fitting                                                      | 1        |
+------------------+--------+----------------------------------------------------------------------------------+----------+

``params``
----------

Configurations for hyperparameters, default values are specified by models.

+-----------------+----------------+---------+-------------------------------------------------------------------+
| Field           | Hyperparameter | Type    | Description                                                       |
+=================+================+=========+===================================================================+
| lr              | Lr             | float64 | learning rate                                                     |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| reg             | Reg            | float64 | regularization strength                                           |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| n_epochs        | NEpochs        | int     | number of epochs                                                  |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| n_factors       | NFactors       | int     | number of factors                                                 |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| random_state    | RandomState    | int     | random state (seed)                                               |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| use_bias        | UseBias        | bool    | using bias                                                        |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| init_mean       | InitMean       | float64 | mean of gaussian initial parameters                               |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| init_std        | InitStdDev     | float64 | standard deviation of gaussian initial parameters                 |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| init_low        | InitLow        | float64 | lower bound of uniform initial parameters                         |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| init_high       | InitHigh       | float64 | upper bound of uniform initial parameters                         |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| n_user_clusters | NUserClusters  | int     | number of user clusters                                           |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| n_item_clusters | NItemClusters  | int     | number of item clusters                                           |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| type            | Type           | string  | type for KNN (``basic``, ``centered``, ``z_score``, ``baseline``) |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| user_based      | UserBased      | bool    | user based if true. otherwise item based                          |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| similarity      | Similarity     | string  | similarity metrics (``pearson``, ``cosine``, ``msd``)             |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| k               | K              | int     | number of neighbors                                               |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| min_k           | MinK           | int     | least number of neighbors                                         |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| optimizer       | Optimizer      | string  | optimizer for optimization (sgd/bpr)                              |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| shrinkage       | Shrinkage      | int     | shrinkage strength of similarity                                  |
+-----------------+----------------+---------+-------------------------------------------------------------------+
| alpha           | Alpha          | float64 | alpha value, depend on context                                    |
+-----------------+----------------+---------+-------------------------------------------------------------------+

Example
=======

.. code-block:: toml

    # This section declares settings for the server.
    [server]
    host = "127.0.0.1"      # server host
    port = 8080             # server port

    # This section declares setting for the database.
    [database]
    file = "gorse.db"       # database file

    # This section declares settings for recommendation.
    [recommend]
    model = "bpr"           # recommendation model
    cache_size = 100        # the number of cached recommendations
    update_threshold = 10   # update model when more than 10 ratings are added
    check_period = 1        # check for update every one minute
    similarity = "pearson"  # similarity metric for neighbors
    fit_jobs = 1            # concurrent jobs for fitting

    # This section declares hyperparameters for the recommendation model.
    [params]
    n_factors = 10          # the number of latent factors
    reg = 0.01              # regularization strength
    lr = 0.05               # learning rate
    n_epochs = 100          # the number of learning epochs
    init_mean = 0.0         # the mean of initial latent factors initilaized by Gaussian distribution
    init_std = 0.001        # the standard deviation of initial latent factors initilaized by Gaussian distribution
