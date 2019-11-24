===============
Baseline Models
===============

Baseline
========

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

Definition
----------

The prediction of baseline model is the sum of global bias :math:`b`, user bias :math:`b_i` and item bias :math:`b_j` :

.. math::

    \hat r_{ij}=b+b_i+b_j

The loss function with regularization is:

.. math::

    \mathcal L=(\hat r_{ij}- r_{ij})^2+\lambda\left(b_i^2+b_j^2\right)



Training
--------

The update rule is:

.. math::

    b&\leftarrow b-\mu(\hat r_{ij}-r_{ij})\\
    b_i&\leftarrow b_i-\mu\left((\hat r_{ij}-r_{ij})+\lambda b_i\right)\\
    b_j&\leftarrow b_j-\mu\left((\hat r_{ij}-r_{ij})+\lambda b_j\right)\\

ItemPop
=======

The only non-personalized model is ItemPop. It always recommmends top K popular items to all users.
