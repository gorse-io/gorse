=========
Slope One
=========

Slope One [#SO]_ predicts ratings by deviations between ratings of each pair of items.

Hyperparameters
===============

There are no hyperparameters for Slope One.

Definition
==========

The ratings from a given user, called an *evaluation*, is represented as an incomplete array :math:`u`, where :math:`u_i` is the rating of this user gives to item :math:`i`. The subset of the set of items consisting of all those items which are rated in u is :math:`S(u)`. The set of all evaluations in the training set is :math:`\chi`.  The number of elements in a set :math:`S` is :math:`card(S)`. The average of ratings in an evaluation u is denoted :math:`\overline u`. The set :math:`S(\chi)` is the set of all evaluations :math:`u\in\chi` such that they contain item :math:`i (i \in S(u))`. Predictions, which we write :math:`P(u)`, represent a vector where each component is the prediction corresponding to one item: predictions depend implicitly on the training set :math:`\chi`.


Training
========

The Slope One [#SO]_ scheme takes into account both information from other users who rated the same item and from the other items rated by the same users.

Given a training set :math:`\chi`, and any two items :math:`j` and :math:`i` with ratings :math:`u_j` and :math:`u_i` respectively in some user evaluation :math:`u \in S_{j, i}(\chi) )`, consider the average deviation if item :math:`i` with respect to :math:`j` as:

.. math::

    \operatorname{dev}_{j, i}=\sum_{u \in S_{j, i}(\chi)} \frac{u_{j}-u_{i}}{\operatorname{card}\left(S_{j, i}(\chi)\right)}


Predict
=======

Given that :math:`\operatorname{dev}_{j, i}+u_{i}` is a prediction for :math:`u_j` given :math:`u_i`, a reasonable predictor might be the average of all such predictions

.. math::

    P(u)_{j}=\frac{1}{\operatorname{card}\left(R_{j}\right)} \sum_{i \in R_{j}}\left(\operatorname{dev}_{j, i}+u_{i}\right)

where :math:`R_{j}=\left\{i | i \in S(u), i \neq j, \operatorname{card}\left(S_{j, i}(\chi)\right)>0\right\}` is the set of all relevant items. For a dense enough data set, that is, where :math:`\operatorname{card}\left(S_{j, i}(\chi)\right)>0` for almost all :math:`i,j`, most of time :math:`R_{j}=S(u)` for :math:`j \notin S(u)` and :math:`R_{j}=S(u)-\{j\}` when :math:`j \in S(u)`. Since :math:`\overline{u}=\sum_{i \in S(u)} \frac{u_{i}}{\operatorname{card}(S(u))} \simeq \sum_{i \in R_{j}} \frac{u_{i}}{\operatorname{card}\left(R_{j}\right)}` for most :math:`j`, simplifying the prediction formula as 


.. math::

    P^{S 1}(u)_{j}=\overline{u}+\frac{1}{\operatorname{card}\left(R_{j}\right)} \sum_{i \in R_{j}} \operatorname{dev}_{j, i}
 

References
==========

.. [#SO] "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.
