=======================
Data Import/Export Tool
=======================

*gorse* provides data import/export tools for data transport between the database and CSV files.

.. code-block:: bash

   gorse export-feedback [database_file] [csv_file] [flags]
   gorse import-feedback [database_file] [csv_file] [flags]
   gorse export-items [database_file] [csv_file] [flags]
   gorse import-items [database_file] [csv_file] [flags]

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