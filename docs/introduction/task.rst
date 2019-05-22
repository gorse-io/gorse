====
Task
====

There are two tasks supported by *gorse*: item ranking and similar recommendation. The rating prediction is also implemented as a previous stage for item ranking when the feedback is explicit.

Rating Prediction
=================

**Given a user and an item, the recommender system is required to predict the rating.** The famous Netflix Prize [#netflix]_ is a competition to predict ratings. It works with explicit feedbacks but might be intractable in practice since there are few explicit feedbacks in the real world.

Definition
^^^^^^^^^^

Predict the rating :math:`\hat r_{ui}` that user :math:`u` gives to item :math:`i`.

Evaluation
^^^^^^^^^^

A rating prediction model will be evaluated on a test dataset :math:`D = \{ <u,i,r_{ui}> \}`.

Root Mean Sqaure Error
""""""""""""""""""""""

.. math::

    \text{RMSE}=\sqrt{\frac{1}{|D|}\sum_{<u,i,r_{ui}>\in D}(\hat r_{ui}-r_{ui})^2}

Mean Absolute Error
"""""""""""""""""""

.. math::

    \text{MAE}=\frac{1}{|D|}\sum_{<u,i,r_{ui}>\in D}|\hat r_{ui}-r_{ui}|

.. warning::

    Rating prediction has been implemented by *gorse*, but serves for item ranking task. You can't get rating prediction directly from RESTful APIs.

Item Ranking
============

**In this case, a list of items are recommended to a user.** For explicit feedbacks, items could be ranked by their predicted ratings. For implicit feedbacks, item ranking could be done by ranking models. The ranking model could be point-wise (WRMF [#WRMF]_), pair-wise (BPR [#BPR]_) or list-wise (CLiMF [#CLiMF]_).

Definition
^^^^^^^^^^

Generate a list of top-N items :math:`\hat I_u` for user :math:`u`, where :math:`N=|\hat I_u|`. The i-th item in the list is :math:`\hat I_{u,i}`.

Evaluation
^^^^^^^^^^

For each user :math:`u`, there is a list of items :math:`I^+_u` (from test dataset) that user really interested in. Items that user :math:`u` has been interacted must be exclued from :math:`\hat I_u` and  :math:`I^+_u`.

Precision
"""""""""

.. math::

    \text{Precision}(u)=\frac{\mid I^+_u\mid\cap\mid \hat I_u \mid}{\mid \hat I_u \mid}

Recall
""""""

.. math::

    \text{Recall}(u)=\frac{\mid I^+_u \mid \cap \mid \hat I_u \mid}{\mid I^+_u \mid}

Mean Reciprocal Rank
""""""""""""""""""""

.. math::

    \text{MRR}(u)=\frac{1}{\mid U\mid}\sum_{u\in U}\frac{1}{\text{rank}(u)}

.. math::

    \text{rank}(u) = \begin{cases}
        \min_{\hat I_{u,i}, \in I^+_u} i & \exists i, \hat I_{u,i} \in I^+_u \\
        +\infty, & \forall i, \hat I_{u,i} \notin I^+_u \\
    \end{cases}

Mean Average Precision
""""""""""""""""""""""

.. math::

    \text{AP}(u)=\frac{1}{|I^+_u|}\sum^N_{i=1}\mathcal 1(\hat I_{u,i} \in I^+_u)\text{Precision}(u)_i

Normalized Discounted Cumulative Gain
"""""""""""""""""""""""""""""""""""""

.. math::

    \text{NDCG}(u)=\frac{1}{N}\sum^N_{i=1}\frac{\mathcal 1(\hat I_{u,i} \in I^+_u)}{\log_2(1+i)}


Similar Recommendation
======================

Recommend similar items ranked by :ref:`_similarity` to a item.

References
==========


.. [#netflix] Bennett, James, and Stan Lanning. "The netflix prize." Proceedings of KDD cup and workshop. Vol. 2007. 2007.

.. [#WRMF] Hu, Yifan, Yehuda Koren, and Chris Volinsky. “Collaborative filtering for implicit feedback datasets.” Data Mining, 2008. ICDM‘08. Eighth IEEE International Conference on. Ieee, 2008.

.. [#BPR] Rendle, Steffen, et al. “BPR: Bayesian personalized ranking from implicit feedback.” Proceedings of the twenty-fifth conference on uncertainty in artificial intelligence. AUAI Press, 2009.

.. [#CLiMF] Shi, Yue, et al. "CLiMF: learning to maximize reciprocal rank with collaborative less-is-more filtering." Proceedings of the sixth ACM conference on Recommender systems. ACM, 2012.
