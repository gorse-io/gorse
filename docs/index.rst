.. gorse documentation master file, created by
   sphinx-quickstart on Sun May 19 13:59:53 2019.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

`gorse` is a recommender system engine consists of two parts and five packages.

<img width=320px src="https://img.sine-x.com/packages.png">

The application layer provides CLI tools for cross-validation, data import, database initialization, and recommendation service. 

The package layer provides components to build a recommender system. Base data structures are included in the `base` package and basic computations are implemented in the `float` package. The `core` package consists of data loader, data splitter, evaluator and model selection tools. All recommendation models are in the `tool` package.

All data are stored in a SQL database. There are five tables in the initialized database:

<img width=420px src="https://img.sine-x.com/database.png">

All items are stored in the `items` table while all ratings are stored in the `ratings` table. Recommendation models load data from the `ratings` table and learn parameters. Then, top n recommendations are cached in the `ratings` table and top n neighbor items are cached in the `neighbors` table. Metadata are saved in the `status` table.

## For Server Users: Setup a Recommender System

## For Package Users: Hack on Recommender System


Contents
========

.. toctree::
   :caption: Table of Contents
   :maxdepth: 1

   introduction/index
   tutorial/index
   usage/index
   develop/index
