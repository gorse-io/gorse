====
Data
====

There are typically two kinds of available feedbacks from users: **explicit feedback** and **implicit feedback**.

Explicit Feedback
=================

**For explicit feedback, users give their preferences by ratings**. For example, a user gives 5 stars to a movie he likes and 1 star to a movie that he dislikes. A user's taste could be represented by ratings that he gives.

The dataset of explicit feedback is defined as:

.. math::

    D = \{ <u,i,r_{ui}> \}

There are multiple tuples :math:`<u,i,r_{ui}>` in the dataset. A tuple is consisted of a user ID :math:`u`, a item ID :math:`i` and a rating :math:`r_{ui}`. The table of a dataset should be:

==== ==== ======
User Item Rating
==== ==== ======
0    1    5
0    2    4
1    1    3
1    2    4
==== ==== ======

Example: [MovieLens]_ 

Implicit Feedback
=================

**For implicit feedback, only positive feedbacks are collected.** For example, implicit feedback represents that a user watched a movie. However, there is no feedback that represents a user dislikes a movie. It's even worse that implicit feedback couldn't show that the user likes the movie. Implicit feedbacks are naturally noisy but it is the common situation in practice.

The dataset of implicit feedback is defined as:

.. math::

    D = \{ <u,i> \}

There are multiple tuples :math:`<u,i>` in the dataset. A tuple is consisted of a user ID :math:`u` and a item ID :math:`i`. The table of a dataset should be:

==== ====
User Item
==== ====
0    1   
0    2   
1    1   
1    2   
==== ====

Example: [Last.FM]_

Weighted Implicit Feedback
==========================

**For weighted implicit feedback, positive feedbacks and their weights are collected.** For example, a user :math:`u` played a game :math:`i` for :math:`r_{ui}` hours. There is no rating that represents this user likes or dislikes this game. The weight :math:`r_{ui}` is a score of confidence rather than preference.

The definition is almost the same as explicit feedback:

.. math::

    D = \{ <u,i,r_{ui}> \}

There are multiple tuples :math:`<u,i,r_{ui}>` in the dataset. A tuple is consisted of a user ID :math:`u`, a item ID :math:`i` and a weight :math:`r_{ui}`. The table of a dataset should be:

==== ==== ======
User Item Weight
==== ==== ======
0    1    100
0    2    20
1    1    5
1    2    50
==== ==== ======

Example: [SteamVideoGames]_

Datasets
========

.. [MovieLens] https://grouplens.org/datasets/movielens/
.. [Last.FM] https://grouplens.org/datasets/hetrec-2011/
.. [SteamVideoGames] https://www.kaggle.com/tamber/steam-video-games
