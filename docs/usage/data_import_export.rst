=======================
Data Import/Export Tool
=======================

*gorse* provides data import/export tools for data transport between the database and CSV files.

Import Items
============

.. code-block:: bash

   gorse import-items [database_file] [csv_file] [flags]

Flags
-----

============ =========================================
Flag         Description
============ =========================================
--header     using a header for the CSV files
--sep string separator for the CSV files (default ",")
-t int       specify the timestamp column
============ =========================================

Example
-------

.. code-block:: bash

   gorse import-items database.db u.item --sep $'|' --t 2

This command import items data from ``u.item`` into ``database.db``. Fields in ``u.item`` are separated by ``|``, there is no header and the 2-th column contains timestamps.

Import Feedback
===============

.. code-block:: bash

   gorse import-feedback [database_file] [csv_file] [flags]

Flags
-----

============ =========================================
Flag         Description
============ =========================================
--header     using a header for the CSV files
--sep string separator for the CSV files (default ",")
============ =========================================

Example
-------

.. code-block:: bash

   gorse import-feedback database.db ratings.dat --sep "::"

This command import feedback data from ``ratings.dat`` into ``database.db``. Fields in ``ratings.dat`` are separated by ``::`` and there is no header.

Export Items
============

.. code-block:: bash

   gorse export-items [database_file] [csv_file] [flags]

Flags
-----

============ =========================================
Flag         Description
============ =========================================
--header     using a header for the CSV files
--sep string separator for the CSV files (default ",")
-t           export with timestamp
============ =========================================

Example
-------

.. code-block:: bash

   gorse export-items database.db items.csv

This command export items data from ``database.db`` into ``items.csv`` with timestamps.

Export Feedback
===============

.. code-block:: bash

   gorse export-feedback [database_file] [csv_file] [flags]

Flags
-----

============ =========================================
Flag         Description
============ =========================================
--header     using a header for the CSV files
--sep string separator for the CSV files (default ",")
============ =========================================

Example
-------

.. code-block:: bash

   gorse export-feedback database.db ratings.csv

This command export feedback data from ``database.db`` into ``ratings.csv``.