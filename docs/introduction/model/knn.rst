===
KNN
===

Neighbors-based models [#KNN]_ predict perference from similar items or similar users. There are two kinds of neighbors-based models: user-based and item-based, which means whether predictions are made by similar users or items. In general, item-based model is better than user-based model since items' characters are more consistent than users' preferences.

.. note::

    In this section, KNN models are introduced by user-based forms. Item-based models could be got by transposing positions of users and items.

KNN for Explicit Feedback
=========================

For explicit feedback, neighbors-based models predict ratings from similar items or similar users.

Hyperparameters
---------------

+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| Key             | Hyperparameter | Type    | Description                                                       | Default |
+=================+================+=========+===================================================================+=========+
| lr              | Lr             | float64 | learning rate (for baseline)                                      | 0.005   |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| reg             | Reg            | float64 | regularization strength (for baseline)                            | 0.02    |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| n_epochs        | NEpochs        | int     | number of epochs (for baseline)                                   | 20      |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| type            | Type           | string  | type for KNN (``basic``, ``centered``, ``z_score``, ``baseline``) | basic   |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| user_based      | UserBased      | bool    | user based if true. otherwise item based                          | true    |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| similarity      | Similarity     | string  | similarity metrics (``pearson``, ``cosine``, ``msd``)             | msd     |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| k               | K              | int     | number of neighbors                                               | 40      |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| min_k           | MinK           | int     | least number of neighbors                                         | 1       |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+
| shrinkage       | Shrinkage      | int     | shrinkage strength of similarity                                  | 100     |
+-----------------+----------------+---------+-------------------------------------------------------------------+---------+

Definition
----------

Items rated by two different users :math:`u` and :math:`v` are represented by :math:`I_u` and :math:`I_v`. Users rated two different items :math:`i` and :math:`j` are represented by :math:`U_i` and :math:`U_j`. The rating that user :math:`u` gives to item :math:`i` is :math:`r_{ui}` and the predicted rating is :math:`\hat r_{ui}`.

.. _similarity:

Similarity
----------

Similarity metrics define nearest neighbors.  There are several most used similarity functions:

Cosine
^^^^^^

.. math::

    \cos(u,v)=\frac{\sum\limits_{k\in|I_u\cap I_v|}r_{uk}\cdot r_{vk}}{\sqrt{\sum\limits_{k\in|I_u\cap I_v|}r_{uk}^2}\cdot\sqrt{\sum\limits_{k\in|I_u\cap I_v|}r_{vk}^2}}

Pearson
^^^^^^^

Pearson similarity is similar to cosine similarity but ratings are subtracted by means first.

.. math::

    \text{pearson}(u,v)=\frac{\sum\limits_{k\in|I_u\cap I_v|}(r_{uk}-\tilde r_u)\cdot (r_{vk}-\tilde r_v)}{\sqrt{\sum\limits_{k\in|I_u\cap I_v|}(r_{uk}-\tilde r_u)^2}\cdot\sqrt{\sum\limits_{k\in|I_u\cap I_v|}(r_{vk}-\tilde r_v)^2}}

where :math:`\tilde r_u` is the mean of ratings rated by the user :math:`u`:

.. math::

    \tilde r_u = \sum_{k\in I_u} r_{uk}

Mean Square Distance
^^^^^^^^^^^^^^^^^^^^


The *Mean Square Distance* is

.. math::

    \text{msd}(u,v)=\frac{1}{|I_u\cap I_v|}\sum_{k\in|I_u\cap I_v|}(r_{uk}-r_{vk})^2

Then, the *Mean Square Distance Similarity* is

.. math::

    \text{MSDSim}(u, v) = \frac{1}{\text{msd}(u, v) + 1}


Predict
-------

A rating could be predict by k nearest neighbors :math:`\mathcal N_k(u)` (k users with top k similarities to user :math:`u`).

.. math::

    \hat r_{ui}=\frac{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)r_{vi}}{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)}


The basic KNN prediction has some problem. There are more advanced methods which achieve higher accuracy.

KNN with Mean
^^^^^^^^^^^^^

Some users tend to give higher ratings but some users tend to give lower ratings. It's resonable to substract ratings with the mean.

.. math::

    \hat r_{ui}=\tilde r_u+\frac{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)(r_{vi}-\tilde r_v)}{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)}

where :math:`\tilde r_v` is the mean of :math:`v`-th user's ratings.
    
KNN with Z-score
^^^^^^^^^^^^^^^^

Different users give ratings with different scales, the solution is to standardlize ratings.

.. math::

    \hat r_{ui}=\tilde r_u+\sigma(r_u)\frac{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)\frac{r_{vj}-\tilde r_v}{\sigma(r_v)}}{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)}

where :math:`\sigma(r_v)` is the standard deviation of :math:`v`-th user's ratings.

KNN with Baseline
^^^^^^^^^^^^^^^^^

Ratings could be substract by biases from baseline model as well.

.. math::

    \hat r_{ui}=b_u+\frac{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)(r_{vi}- b_v)}{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)}

where :math:`b_u` is the bias comes from the baseline model :math:`\hat r_{ui}=b+b_u+b_i`. The KNN model with baseline is the best model since biases are used.


KNN for Implicit Feedback
=========================

For implicit feedback [#BPR]_, neighbors-based models predict wheater a users will interact with a item from similar items or similar users.

Hyperparameters
---------------

There are no hyperparameters for implicit version KNN.

Definition
----------

Items interacted with user :math:`u`  are represented by :math:`I^+_u`. Users interacted with item :math:`i` are represented by :math:`U^+_i`. The confidence of predicting user :math:`u` will interact with item :math:`i` is :math:`\hat x_{ui}`.

Similarity
----------

The similarity for implicit feedback is slightly different.

.. math::

    c_{i, j}^{\operatorname{cosine}} :=\frac{\left|U_{i}^{+} \cap U_{j}^{+}\right|}{\sqrt{\left|U_{i}^{+}\right| \cdot\left|U_{j}^{+}\right|}}

Predict
-------

The prediction is given by the sum of similarities.

.. math::

    \hat{x}_{u i}=\sum_{l \in I_{u}^{+} \wedge l \neq i} c_{i l}

References
==========

.. [#KNN] Desrosiers, Christian, and George Karypis. "A comprehensive survey of neighborhood-based recommendation methods." Recommender systems handbook. Springer, Boston, MA, 2011. 107-144.

.. [#BPR] Rendle, Steffen, et al. "BPR: Bayesian personalized ranking from implicit feedback." Proceedings of the twenty-fifth conference on uncertainty in artificial intelligence. AUAI Press, 2009.
