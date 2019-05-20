====
Task
====

Rating Prediction
=================

**Given a user and an item, the recommender system is required to predict the rating.** The famous Netflix Prize [#netflix]_ is a competition to predict ratings. It works with explicit feedbacks but might be intractable in practice since there are few explicit feedbacks in the real world.

.. warning::

    Rating prediction has been implemented by *gorse*, but serves for item ranking task. You can't get rating prediction directly from RESTful APIs.

Item Ranking
============

**In this case, a list of items are recommended to a user.** For explicit feedbacks, items could be ranked by their predicted ratings. For implicit feedbacks, item ranking could be done by ranking models. The ranking model could be point-wise (WRMF [#WRMF]_), pair-wise (BPR [#BPR]_) or list-wise (CLiMF [#CLiMF]_).


Similar Recommendation
======================

References
==========


.. [#netflix] Bennett, James, and Stan Lanning. "The netflix prize." Proceedings of KDD cup and workshop. Vol. 2007. 2007.

.. [#WRMF] Hu, Yifan, Yehuda Koren, and Chris Volinsky. “Collaborative filtering for implicit feedback datasets.” Data Mining, 2008. ICDM‘08. Eighth IEEE International Conference on. Ieee, 2008.

.. [#BPR] Rendle, Steffen, et al. “BPR: Bayesian personalized ranking from implicit feedback.” Proceedings of the twenty-fifth conference on uncertainty in artificial intelligence. AUAI Press, 2009.

.. [#CLiMF] Shi, Yue, et al. "CLiMF: learning to maximize reciprocal rank with collaborative less-is-more filtering." Proceedings of the sixth ACM conference on Recommender systems. ACM, 2012.
