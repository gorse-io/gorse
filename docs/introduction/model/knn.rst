===
KNN
===

KNN
---

Neighbors-based models [#KNN]_ predict ratings from similar items or similar users. There are two kinds of neighbors-based models: user-based and item-based, which means whether predictions are made by similar users or items. In general, item-based model is better than user-based model since items' characters are more consistent than users' preferences.

.. _similarity:

Similarity
^^^^^^^^^^

Similarity metrics define the definition of nearest neighbors. Items rated by two different users :math:`u` and :math:`v` are represented by :math:`I_u` and :math:`I_v`. Users rated two different items :math:`i` and :math:`j` are represented by :math:`U_i` and :math:`U_j`. There are several most used similarity functions:

Cosine
""""""

.. math::

    \cos(u,v)=\frac{\sum\limits_{k\in|I_u\cap I_v|}r_{uk}\cdot r_{vk}}{\sqrt{\sum\limits_{k\in|I_u\cap I_v|}r_{uk}^2}\cdot\sqrt{\sum\limits_{k\in|I_u\cap I_v|}r_{vk}^2}}

Pearson
"""""""

Pearson similarity is similar to cosine similarity but ratings are subtracted by means first.

.. math::

    \text{pearson}(a,b)=\frac{\sum\limits_{k\in|I_a\cap I_b|}(r_{ak}-\tilde r_a)\cdot (r_{bk}-\tilde r_b)}{\sqrt{\sum\limits_{k\in|I_a\cap I_b|}(r_{ak}-\tilde r_a)^2}\cdot\sqrt{\sum\limits_{k\in|I_a\cap I_b|}(r_{bk}-\tilde r_b)^2}}

where :math:`\tilde r_a` is the mean of ratings rated by the user :math:`a`:

.. math::

    \tilde r_a = \sum_{k\in I_a} r_{ak}

Mean Square Distance
""""""""""""""""""""


The *Mean Square Distance* is

.. math::

    \text{msd}(a,b)=\frac{1}{|I_a\cap I_b|}\sum_{k\in|I_a\cap I_b|}(r_{ak}-r_{bk})^2

Then, the *Mean Square Distance Similarity* is

.. math::

    \text{MSDSim}(u, v) = \frac{1}{\text{msd}(u, v) + 1}


Predict
^^^^^^^

A rating could be predict by k nearest neighbors $\mathcal N_k(i)$ (k users or items with max k similarity).

.. math::

    \hat r_{ui}=\frac{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)r_{vi}}{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)}


The basic KNN prediction has some problem, There are more advanced methods which achieve higher accuracy.

KNN with Mean
"""""""""""""

$\tilde r_l$ is the mean of l-th user's (or item's) ratings.

.. math::

    \hat r_{ij}=\tilde r_i+\frac{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)(r_{lj}-\tilde r_l)}{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)}
    
KNN with Z-score
""""""""""""""""

$\sigma(r_i)$ is the standard deviation of l-th user's (or item's) ratings.

.. math::

    \hat r_{ij}=\tilde r_i+\sigma(r_i)\frac{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)\frac{r_{lj}-\tilde r_l}{\sigma(r_l)}}{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)}


KNN with Baseline
"""""""""""""""""

$b_l$ is the baseline comes from the baseline model $\hat r_{ij}=b+b_i+b_j+p_i^Tq_j$.

.. math::

    \hat r_{ij}=b_i+\frac{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)(r_{lj}- b_l)}{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)}

The KNN model with baseline is the best model since biases are used.



References
==========

.. [#KNN] Desrosiers, Christian, and George Karypis. "A comprehensive survey of neighborhood-based recommendation methods." Recommender systems handbook. Springer, Boston, MA, 2011. 107-144.
