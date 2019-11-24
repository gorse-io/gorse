=====================================================
Build a Steam Recommender System with Flask and gorse
=====================================================

Recommender system has now affected or even controlled people’s lives. This article will start with the simplest recommendation algorithms, to build a Steam games recommender system using `Flask <https://github.com/pallets/flask>`_ and `gorse <https://github.com/zhenghaoz/gorse>`_.

.. image:: https://img.sine-x.com/steamlens-demo-view.png
  :width: 500

The Architecture of Recommender System
======================================

The architecture of our recommender system as shown below:

.. image:: https://img.sine-x.com/steamlens-architecture.png
  :width: 500

There are three parts:

- *gorse*: gorse is an offline recommender system that recieves user-game purchase records, then trains the recommender model automatically and generates a list of recommended games.

- *Flask*: A Web service written in Flask are responsible for user login, requesting users’ owned games from Steam, pushing owned games to gorse and pulling recommended games from gorse.

- *Steam*: Provides users’ owned games via the API and provides game cover images.

This Steam games recommender system has been deployed at `https://steamlens.gorse.io <https://steamlens.gorse.io>`_. Anyone have a Steam account could try out its personalized recommendations. The code has been shared on GitHub. so anyone could try to deploy it.

Setup a Recommender System Server
=================================

Installation
------------

We need to install gorse, the recommender system backend. If Golang has been installed and ``$GOBIN`` has been added to the environment variable $PATH, we can install it directly using the following command:

.. code-block:: bash

  $ go get github.com/zhenghaoz/gorse/...

Data Preparation
----------------

Recommender system doesn’t work without data. Fortunately, there are `Steam datasets <https://steam.internet.byu.edu/>`_ shared by others on the Internet. The original dataset is huge so it has to be sampled into `games.csv <http://cdn.sine-x.com/backups/games.csv>`_. We create a folder for this project and then download the data:

.. code-block:: bash

    $ mkdir SteamLens
    $ cd SteamLens
    $ wget http://cdn.sine-x.com/backups/games.csv
    ...
    $ head games.csv
    76561197960272226,10,505
    76561197960272226,20,0
    76561197960272226,30,0
    76561197960272226,40,0
    76561197960272226,50,0
    76561197960272226,60,0
    76561197960272226,70,0
    76561197960272226,130,0
    76561197960272226,80,0
    76561197960272226,100,0

We can find that the data has three columns, which are user, game, and played times.

Test Models
-----------

Before setup a recommender service, we need to choose the best recommender model. gorse provides the tool to evaluate multiple models. We can run gorse test -h or check `online documents <https://gorse.readthedocs.io/en/latest/usage/cross_validation.html>`_ to learn its usage. Our datasets are implicitly weighted (played time) and can be used by four models according to `the input supported by each model <https://gorse.readthedocs.io/en/latest/introduction/model/index.html>`_: item-pop, knn-implicit, bpr, and wrmf.

- Test the item-pop model:

.. code-block:: bash

    $ gorse test item-pop --load-csv games.csv --csv-sep ',' --eval-precision --eval-recall --eval-ndcg --eval-map --eval-mrr
    ...
    +--------------+----------+----------+----------+----------+----------+----------------------+
    |              |  FOLD 1  |  FOLD 2  |  FOLD 3  |  FOLD 4  |  FOLD 5  |         MEAN         |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    | Precision@10 | 0.080942 | 0.080655 | 0.080253 | 0.078880 | 0.078248 | 0.079796(±0.001548)  |
    | Recall@10    | 0.308894 | 0.310532 | 0.312299 | 0.305665 | 0.308428 | 0.309163(±0.003498)  |
    | NDCG@10      | 0.211919 | 0.209796 | 0.209004 | 0.209945 | 0.210466 | 0.210226(±0.001693)  |
    | MAP@10       | 0.133684 | 0.132018 | 0.130520 | 0.133500 | 0.135297 | 0.133004(±0.002484)  |
    | MRR@10       | 0.247601 | 0.242664 | 0.240176 | 0.244244 | 0.241920 | 0.243321(±0.004280)  |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    2019/11/07 09:56:51 Complete cross validation (22.037387763s)

- Test the knn_implicit model:

.. code-block:: bash

    $ gorse test knn_implicit --load-csv games.csv --csv-sep ',' --eval-precision --eval-recall --eval-ndcg --eval-map --eval-mrr
    ...
    +--------------+----------+----------+----------+----------+----------+----------------------+
    |              |  FOLD 1  |  FOLD 2  |  FOLD 3  |  FOLD 4  |  FOLD 5  |         MEAN         |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    | Precision@10 | 0.150892 | 0.153211 | 0.147429 | 0.152162 | 0.150013 | 0.150742(±0.003312)  |
    | Recall@10    | 0.529160 | 0.546523 | 0.533619 | 0.543382 | 0.533702 | 0.537277(±0.009245)  |
    | NDCG@10      | 0.528442 | 0.546386 | 0.529590 | 0.545167 | 0.530433 | 0.536004(±0.010383)  |
    | MAP@10       | 0.451220 | 0.469989 | 0.453748 | 0.468641 | 0.453865 | 0.459493(±0.010497)  |
    | MRR@10       | 0.635610 | 0.656008 | 0.636238 | 0.658769 | 0.636045 | 0.644534(±0.014235)  |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    2019/11/07 09:59:14 Complete cross validation (1m4.169339752s)

- Test the bpr model:

.. code-block:: bash

    $ gorse test bpr --load-csv games.csv --csv-sep ',' --eval-precision --eval-recall --eval-ndcg --eval-map --eval-mrr
    ...
    +--------------+----------+----------+----------+----------+----------+----------------------+
    |              |  FOLD 1  |  FOLD 2  |  FOLD 3  |  FOLD 4  |  FOLD 5  |         MEAN         |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    | Precision@10 | 0.127123 | 0.128440 | 0.129396 | 0.124914 | 0.126719 | 0.127318(±0.002405)  |
    | Recall@10    | 0.502971 | 0.511863 | 0.515385 | 0.503914 | 0.505500 | 0.507926(±0.007458)  |
    | NDCG@10      | 0.434958 | 0.421336 | 0.427279 | 0.405582 | 0.424385 | 0.422708(±0.017126)  |
    | MAP@10       | 0.350960 | 0.332219 | 0.336659 | 0.313238 | 0.337824 | 0.334180(±0.020942)  |
    | MRR@10       | 0.495087 | 0.466407 | 0.477137 | 0.447885 | 0.475176 | 0.472338(±0.024453)  |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    2019/11/07 10:01:51 Complete cross validation (56.85278659s)

- Test the wrmf model (with :math:`\alpha = 0.001`):

.. code-block:: bash

    $ gorse test wrmf --load-csv games.csv --csv-sep ',' --eval-precision --eval-recall --eval-ndcg --eval-map --eval-mrr --set-alpha 0.001
    ...
    +--------------+----------+----------+----------+----------+----------+----------------------+
    |              |  FOLD 1  |  FOLD 2  |  FOLD 3  |  FOLD 4  |  FOLD 5  |         MEAN         |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    | Precision@10 | 0.145834 | 0.148021 | 0.147034 | 0.146564 | 0.143163 | 0.146123(±0.002960)  |
    | Recall@10    | 0.524673 | 0.533390 | 0.533113 | 0.535772 | 0.525784 | 0.530546(±0.005873)  |
    | NDCG@10      | 0.499655 | 0.504544 | 0.506967 | 0.513855 | 0.501728 | 0.505350(±0.008505)  |
    | MAP@10       | 0.415299 | 0.419840 | 0.423166 | 0.431339 | 0.421243 | 0.422177(±0.009161)  |
    | MRR@10       | 0.592257 | 0.592858 | 0.596109 | 0.610589 | 0.590023 | 0.596367(±0.014222)  |
    +--------------+----------+----------+----------+----------+----------+----------------------+
    2019/11/07 10:06:52 Complete cross validation (3m52.912709237s)

It seems that the KNN mod performs best on our dataset (actually we haven’t pay much attention to tuning other models) and the speed is pleasant, so we choose KNN as the recommender model. No recommender model performs best all the time. The type of best model depends on the characteristics of the dataset. For example, the best model on the MovieLens 100K is WRMF instead of KNN.

Import Data
-----------

After the best model is chosen, we import the data into gorse’s built-in database, create a folder data and import the data into data/gorse.db.

.. code-block:: bash

    $ mkdir data
    $ gorse import-feedback data/gorse.db games.csv --sep ','

Start the Server
----------------

Next, create the configuration file at config/gorse.toml, we need to set the listening address, port, database file location and some recommendation options. The implicit KNN does not need hyper-parameters, so the [params] section is left blank.

.. code-block:: ini

    # This section declares settings for the server.
    [server]
    host = "0.0.0.0"        # server host
    port = 8080             # server port

    # This section declares setting for the database.
    [database]
    file = "data/gorse.db"  # database file

    # This section declares settings for recommendation.
    [recommend]
    model = "knn_implicit"  # recommendation model
    cache_size = 100        # the number of cached recommendations
    update_threshold = 10   # update model when more than 10 ratings are added
    check_period = 1        # check for update every one minute
    similarity = "implicit" # similarity metric for neighbors

    # This section declares hyperparameters for the recommendation model.
    [params]

Save the configuration file and run the recommender server:

.. code-block:: bash

    $ gorse serve -c config/gorse.toml
    ...
    2019/11/07 16:45:05 update recommends
    2019/11/07 16:47:02 update neighbors by implicit

The content in last two lines indicate recommendation results has been generated.

Test Recommendation
-------------------

We can use the `RESTful API <https://gorse.readthedocs.io/en/latest/usage/server/api.html>`_ provided by gorse to get the recommended games:

.. code-block:: bash

    $ curl http://127.0.0.1:8080/recommends/76561197960272226?number=10
    [
        {
            "ItemId": 4540,
            "Score": 23.479386364078838
        },
        ...
        {
            "ItemId": 57300,
            "Score": 22.156954153653245
        }
    ]

We get 10 recommended games with game IDs and ratings.

Create a Front-end Web Server
=============================

Apply API key
-------------

We need to connect to the user’s Steam account to get his or her owned games and it requires user’s authorization. So we need to visit the `“Register Steam Web API Key” <https://steamcommunity.com/dev/apikey>`_ page to apply for an API key for Steam web API.

.. image:: https://img.sine-x.com/steamlens-api-key.png
  :width: 500

Install Python Packages
-----------------------

We install Python packages needed for development:

.. code-block:: bash

    $ pip install Flask
    $ pip install Flask-OpenID
    $ pip install Flask-SQLAlchemy
    $ pip install uWSGI

Create a folder steamlens under SteamLens to place source files:

.. code-block:: bash

    $ mkdir steamlens

Web Page
--------

The design of web pages is not the focus of this article, the HTML template can be seen in `steamlens/templates <https://zhenghaoz.github.io/steam-games-recommender-system/steamlens/templates>`_, the static files can be seen in `steamlens/static <https://zhenghaoz.github.io/steam-games-recommender-system/steamlens/static>`_, and two HTML templates are provided:

+---------------------------------------------------------------------------------------------------------------------+-------------------------------+-----------------------------------------------------------------------------------------------------+
| Template                                                                                                            | Function                      | Data                                                                                                |
+=====================================================================================================================+===============================+=====================================================================================================+
| `page_gallery.jinja2 <https://github.com/zhenghaoz/SteamLens/blob/master/steamlens/templates/page_gallery.jinja2>`_ |	Show a collection of games    |	current_time: time, title: title, items: games, nickname: user’s nickname                           |
+---------------------------------------------------------------------------------------------------------------------+-------------------------------+-----------------------------------------------------------------------------------------------------+
| `page_app.jinja2 <https://github.com/zhenghaoz/SteamLens/blob/master/steamlens/templates/page_gallery.jinja2>`_     |	Show a game and similar games |	current_time: time, item_id: game ID, title: title, items: similar games, nickname: user’s nickname |
+---------------------------------------------------------------------------------------------------------------------+-------------------------------+-----------------------------------------------------------------------------------------------------+

Configuration File for Web Service
----------------------------------

Create the configuration file config/steamlens.cfg for the Flask application before writing codes:

.. code-block:: python 

    # Configuration for gorse
    GORSE_API_URI = 'http://127.0.0.1:8080'
    GORSE_NUM_ITEMS = 30

    # Configuration for SQL
    SQLALCHEMY_DATABASE_URI = 'sqlite:///../data/steamlens.db'
    SQLALCHEMY_TRACK_MODIFICATIONS = False

    # Configuration for OpenID
    OPENID_STIRE = '../data/openid_store'
    SECRET_KEY = 'STEAM_API_KEY'

Don’t forget to replace ``STEAM_API_KEY`` with the Steam API key.

User Login
----------

We first build the basic framework and connect to Steam, the source file is located at steamlens/app.py, the application works as follows:

1. Create a Flask app object and read the configuration file from the environment variable ``STEAMLENS_SETTINGS``;
#. Create an OpenID object to connect to Steam authorization;
#. Create a SQLAlchemy object to connect to the database;
#. After a user logged in, his or her nickname and ID are saved to the database, and his or her owned games are pushed to the gorse server.

.. code-block:: python 

    import json
    import os.path
    import re
    from datetime import datetime
    from urllib.parse import urlencode
    from urllib.request import urlopen

    import requests
    from flask import Flask, render_template, redirect, session, g
    from flask_openid import OpenID
    from flask_sqlalchemy import SQLAlchemy

    app = Flask(__name__)
    app.config.from_envvar('STEAMLENS_SETTINGS')

    oid = OpenID(app, os.path.join(os.path.dirname(__file__), app.config['OPENID_STIRE']))
    db = SQLAlchemy(app)

    #################
    # Steam Service #
    #################

    class User(db.Model):
        id = db.Column(db.Integer, primary_key=True)
        steam_id = db.Column(db.String(40))
        nickname = db.Column(db.String(80))

        @staticmethod
        def get_or_create(steam_id):
            rv = User.query.filter_by(steam_id=steam_id).first()
            if rv is None:
                rv = User()
                rv.steam_id = steam_id
                db.session.add(rv)
            return rv


    @app.route("/login")
    @oid.loginhandler
    def login():
        if g.user is not None:
            return redirect(oid.get_next_url())
        else:
            return oid.try_login("http://steamcommunity.com/openid")


    @app.route('/logout')
    def logout():
        session.pop('user_id', None)
        return redirect('/pop')


    @app.before_request
    def before_request():
        g.user = None
        if 'user_id' in session:
            g.user = User.query.filter_by(id=session['user_id']).first()


    @oid.after_login
    def new_user(resp):
        _steam_id_re = re.compile('steamcommunity.com/openid/id/(.*?)$')
        match = _steam_id_re.search(resp.identity_url)
        g.user = User.get_or_create(match.group(1))
        steamdata = get_user_info(g.user.steam_id)
        g.user.nickname = steamdata['personaname']
        db.session.commit()
        session['user_id'] = g.user.id
        # Add games to gorse
        games = get_owned_games(g.user.steam_id)
        data = [{'UserId': int(g.user.steam_id), 'ItemId': int(v['appid']), 'Feedback': float(v['playtime_forever'])} for v in games]
        headers = {"Content-Type": "application/json"}
        requests.put('http://127.0.0.1:8080/feedback', data=json.dumps(data), headers=headers)
        return redirect(oid.get_next_url())


    def get_user_info(steam_id):
        options = {
            'key': app.secret_key,
            'steamids': steam_id
        }
        url = 'http://api.steampowered.com/ISteamUser/' \
            'GetPlayerSummaries/v0001/?%s' % urlencode(options)
        rv = json.load(urlopen(url))
        return rv['response']['players']['player'][0] or {}


    def get_owned_games(steam_id):
        options = {
            'key': app.secret_key,
            'steamid': steam_id,
            'format': 'json'
        }
        url = 'http://api.steampowered.com/IPlayerService/GetOwnedGames/v0001/?%s' % urlencode(options)
        rv = json.load(urlopen(url))
        return rv['response']['games']


    # Create tables if not exists.
    db.create_all()

Recommend Games
---------------

Then add recommendation pages in steamlens/app.py, use `RESTful API <https://gorse.readthedocs.io/en/latest/usage/server/api.html>`_ provided by gorse to get popular games, random games, personalized recommended games and similar games for a certain game.

.. code-block:: python 

    #######################
    # Recommender Service #
    #######################

    @app.context_processor
    def inject_current_time():
        return {'current_time': datetime.utcnow()}


    @app.route('/')
    def index():
        return redirect('/pop')


    @app.route('/pop')
    def pop():
        # Get nickname
        nickname = None
        if g.user:
            nickname = g.user.nickname
        # Get items
        r = requests.get('%s/popular?number=%d' % (app.config['GORSE_API_URI'], app.config['GORSE_NUM_ITEMS']))
        items = [v['ItemId'] for v in r.json()]
        # Render page
        return render_template('page_gallery.jinja2', title='Popular Games', items=items, nickname=nickname)


    @app.route('/random')
    def random():
        # Get nickname
        nickname = None
        if g.user:
            nickname = g.user.nickname
        # Get items
        r = requests.get('%s/random?number=%d' % (app.config['GORSE_API_URI'], app.config['GORSE_NUM_ITEMS']))
        items = [v['ItemId'] for v in r.json()]
        # Render page
        return render_template('page_gallery.jinja2', title='Random Games', items=items, nickname=nickname)


    @app.route('/recommend')
    def recommend():
        # Check login
        if g.user is None:
            return render_template('page_gallery.jinja2', title='Please login first', items=[])
        # Get items
        r = requests.get('%s/recommends/%s?number=%s' %
                        (app.config['GORSE_API_URI'], g.user.steam_id, app.config['GORSE_NUM_ITEMS']))
        # Render page
        if r.status_code == 200:
            items = [v['ItemId'] for v in r.json()]
            return render_template('page_gallery.jinja2', title='Recommended Games', items=items, nickname=g.user.nickname)
        return render_template('page_gallery.jinja2', title='Generating Recommended Games...', items=[], nickname=g.user.nickname)


    @app.route('/item/<int:app_id>')
    def item(app_id: int):
        # Get nickname
        nickname = None
        if g.user:
            nickname = g.user.nickname
        # Get items
        r = requests.get('%s/neighbors/%d?number=%d' %
                        (app.config['GORSE_API_URI'], app_id, app.config['GORSE_NUM_ITEMS']))
        items = [v['ItemId'] for v in r.json()]
        # Render page
        return render_template('page_app.jinja2', item_id=app_id, title='Similar Games', items=items, nickname=nickname)


    @app.route('/user')
    def user():
        # Check login
        if g.user is None:
            return render_template('page_gallery.jinja2', title='Please login first', items=[])
        # Get items
        r = requests.get('%s/user/%s' % (app.config['GORSE_API_URI'], g.user.steam_id))
        # Render page
        if r.status_code == 200:
            items = [v['ItemId'] for v in r.json()]
            return render_template('page_gallery.jinja2', title='Owned Games', items=items, nickname=g.user.nickname)
        return render_template('page_gallery.jinja2', title='Synchronizing Owned Games ...', items=[], nickname=g.user.nickname)

Start the Web Server
--------------------

We use uWSGI to start the Flask application, a uwsgi.ini is required in the SteamLens folder.

.. code-block:: ini 

    [uwsgi]

    # Bind to the specified UNIX/TCP socket using default protocol
    socket=0.0.0.0:5000

    # Point to the main directory of the Web Site
    chdir=/path/to/SteamLens/steamlens/

    # Python startup file
    wsgi-file=app.py

    # The application variable of Python Flask Core Oject 
    callable=app

    # The maximum numbers of Processes
    processes=1

    # The maximum numbers of Threads
    threads=2

    # Set internal buffer size 
    buffer-size=8192

Remember to change chdir to the path where the folder SteamLens/steamlens is located. Finally run the following command to start the Flask application:

.. code-block:: bash 

    $ STEAMLENS_SETTINGS ../config/steamlens.cfg uwsgi --ini uwsgi.ini

We can check the online demo at `https://steamlens.gorse.io <https://steamlens.gorse.io>`_, login to the system and wait a few minutes for the system to generate personalized recommendations. The recommended games for me are as follows:

.. image:: https://img.sine-x.com/steamlens-full-page.png
  :width: 500

I love FPS games and it recommends a lot of FPS games for me. However, you can find that the recommended games are outdated. This is because the dataset used by this project is crawled about five years ago. As Steam updated its privacy policy, now it is impossible to crawl users’ owned games without users’ authorization.