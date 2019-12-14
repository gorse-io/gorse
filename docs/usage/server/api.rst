===
API
===

Insert
======

Insert New Items
----------------

.. code-block:: bash

   curl -X PUT -H 'Content-Type: application/json' 127.0.0.1:8080/items -d '[2048,2049,2050]'


Insert three new items ``[2048,2049,2050]``.

.. code-block:: json

    {
        "ItemsBefore": 1682,
        "ItemsAfter": 1685,
        "UsersBefore": 221,
        "UsersAfter": 221,
        "FeedbackBefore": 100000,
        "FeedbackAfter": 100000
    }

Insert New Feedbacks
--------------------

.. code-block:: bash

    curl -X PUT -H 'Content-Type: application/json' 127.0.0.1:8080/ratings \
        -d '[{"UserId":2048,"ItemId":1000,"Rating":3},{"UserId":2049,"ItemId":1000,"Rating":3},{"UserId":2050,"ItemId":1000,"Rating":3}]'

Insert three new ratings: ``<2048,1000,3>``, ``<2049,1000,3>`` and ``<2050,1000,3>``.

.. code-block:: json

    {
        "ItemsBefore": 1682,
        "ItemsAfter": 1682,
        "UsersBefore": 221,
        "UsersAfter": 224,
        "FeedbackBefore": 100000,
        "FeedbackAfter": 100003
    }


Query
=====

Get System Status
-----------------

.. code-block:: bash

    curl 127.0.0.1:8080/status

Get current system status.

.. code-block:: json

    {
        "FeedbackCount": 100000,
        "ItemCount": 1682,
        "UserCount": 221,
        "CommitCount": 100000,
        "CommitTime": "2019-12-07 14:47:53.657515767 +0800 CST m=+60.702418740"
    }

Get Popular Items
-----------------

.. code-block:: bash

    curl 127.0.0.1:8080/popular?number=10

Get top 10 popular items.

.. code-block:: json

    [
        {
            "ItemId": 50,
            "Score": 583
        },
        {
            "ItemId": 258,
            "Score": 509
        },
        ...
        {
            "ItemId": 121,
            "Score": 429
        }
    ]

Get Random Items
----------------

.. code-block:: bash

    curl 127.0.0.1:8080/random?number=10

Get 10 random items.

.. code-block:: json

    [
        {
            "ItemId": 42,
            "Score": 0
        },
        {
            "ItemId": 71,
            "Score": 0
        },
        ...
        {
            "ItemId": 1144,
            "Score": 0
        }
    ]


Get Recommended Items
---------------------

.. code-block:: bash

    curl 127.0.0.1:8080/recommends/100/?number=10


Recommend 10 items for the 100th user.

.. code-block:: json

    [
        {
            "ItemId": 748,
            "Score": 1.41319013391051
        },
        {
            "ItemId": 307,
            "Score": 1.39176267281106
        },
        ...
        {
            "ItemId": 259,
            "Score": 1.1303975574071174
        }
    ]

Get Similar Items
-----------------

.. code-block:: bash

    curl 127.0.0.1:8080/neighbors/100/?number=10


Get top 10 similar items for the 100th item.

.. code-block:: json

    [
        {
            "ItemId": 1424,
            "Score": 1.0000000000000002
        },
        {
            "ItemId": 1234,
            "Score": 1
        },
        ...
        {
            "ItemId": 1554,
            "Score": 1
        }
    ]

Get User History
----------------

.. code-block:: bash

    curl 127.0.0.1:8080/user/1

Get interaction history of user 1.

.. code-block:: json

    [
        {
            "ItemId": 1,
            "Score": 5
        },
        ...
    ]


Get all users
-------------

.. code-block:: bash

    curl 127.0.0.1:8080/users

List all users.

.. code-block:: json

    [
        1,
        2,
        ...
    ]
