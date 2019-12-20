===
API
===

Insert
======

Insert New Items
----------------

.. code-block:: bash

    curl -X PUT -H 'Content-Type: application/json' 127.0.0.1:8080/items \
        -d '[{"ItemId":"2048","Timestamp":"2010-1-1"},{"ItemId":"2049","Timestamp":"2011-1-1"},{"ItemId":"2050","Timestamp":"2012-1-1"}]'


Insert three new items ``[{"ItemId":"2048","Timestamp":"2010-1-1"},{"ItemId":"2049","Timestamp":"2011-1-1"},{"ItemId":"2050","Timestamp":"2012-1-1"}]``.

.. code-block:: json

    {
        "ItemsBefore": 1682,
        "ItemsAfter": 1685,
        "UsersBefore": 221,
        "UsersAfter": 221,
        "FeedbackBefore": 100000,
        "FeedbackAfter": 100000
    }

Insert New Feedback
--------------------

.. code-block:: bash

    curl -X PUT -H 'Content-Type: application/json' 127.0.0.1:8080/feedback \
        -d '[{"UserId":"2048","ItemId":"1000","Rating":3},{"UserId":"2049","ItemId":"1000","Rating":3},{"UserId":"2050","ItemId":"1000","Rating":3}]'

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

- ``number``: number of returned items.

.. code-block:: json

    [
        {
            "ItemId": "50",
            "Popularity": 583,
            "Timestamp": "1977-01-01T00:00:00Z",
            "Score": 583
        },
        {
            "ItemId": "258",
            "Popularity": 509,
            "Timestamp": "1997-07-11T00:00:00Z",
            "Score": 509
        },
        ...
        {
            "ItemId": "121",
            "Popularity": 429,
            "Timestamp": "1996-07-03T00:00:00Z",
            "Score": 429
        }
    ]


Get Latest Items
-----------------

.. code-block:: bash

    curl 127.0.0.1:8080/latest?number=10

Get top 10 latest items.

- ``number``: number of returned items.

.. code-block:: json

    [
        {
            "ItemId": "315",
            "Popularity": 160,
            "Timestamp": "1998-10-23T00:00:00Z",
            "Score": 909100800
        },
        {
            "ItemId": "1432",
            "Popularity": 3,
            "Timestamp": "1998-10-09T00:00:00Z",
            "Score": 907891200
        },
        ...
        {
            "ItemId": "1648",
            "Popularity": 1,
            "Timestamp": "1998-03-20T00:00:00Z",
            "Score": 890352000
        }
    ]

Get Random Items
----------------

.. code-block:: bash

    curl 127.0.0.1:8080/random?number=10

Get 10 random items.

- ``number``: number of returned items.

.. code-block:: json

    [
        {
            "ItemId": "42",
            "Popularity": 148,
            "Timestamp": "1994-01-01T00:00:00Z",
            "Score": 0
        },
        {
            "ItemId": "71",
            "Popularity": 220,
            "Timestamp": "1994-01-01T00:00:00Z",
            "Score": 0
        },
        ...
        {
            "ItemId": "1144",
            "Popularity": 3,
            "Timestamp": "1997-05-02T00:00:00Z",
            "Score": 0
        }
    ]

Get Recommended Items
---------------------

.. code-block:: bash

    curl 127.0.0.1:8080/recommends/100/?number=10&p=0.5&t=0.5&c=1

Recommend 10 items for the 100th user.

- ``number``: number of returned items.
- ``p``: weight of popularity.
- ``t``: weight of timestamp.
- ``c``: weight of collaborative filtering.

.. code-block:: json

    [
        {
            "ItemId": "748",
            "Popularity": 316,
            "Timestamp": "1997-03-14T00:00:00Z",
            "Score": 1.41319013391051
        },
        {
            "ItemId": "307",
            "Popularity": 188,
            "Timestamp": "1997-01-01T00:00:00Z",
            "Score": 1.39176267281106
        },
        ...
        {
            "ItemId": "259",
            "Popularity": 162,
            "Timestamp": "1997-01-01T00:00:00Z",
            "Score": 1.1303975574071174
        }
    ]

Get Similar Items
-----------------

.. code-block:: bash

    curl 127.0.0.1:8080/neighbors/100/?number=10


Get top 10 similar items for the 100th item.

- ``number``: number of returned items.

.. code-block:: json

    [
        {
            "ItemId": "1424",
            "Popularity": 3,
            "Timestamp": "1994-01-01T00:00:00Z",
            "Score": 1.0000000000000002
        },
        {
            "ItemId": "1234",
            "Popularity": 8,
            "Timestamp": "1998-01-01T00:00:00Z",
            "Score": 1
        },
        ...
        {
            "ItemId": "1554",
            "Popularity": 2,
            "Timestamp": "1994-01-01T00:00:00Z",
            "Score": 1
        }
    ]

Get User History
----------------

.. code-block:: bash

    curl 127.0.0.1:8080/user/1/feedback

Get interaction history of user 1.

.. code-block:: json

    [
        {
            "UserId": "1",
            "ItemId": "1",
            "Rating": 5
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
        "1",
        "2",
        ...
    ]
