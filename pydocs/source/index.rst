.. pymortar documentation master file, created by
   sphinx-quickstart on Tue Mar 19 10:37:33 2019.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

Welcome to pymortar's documentation!
====================================

PyMortar is a handy-dandy library for Python 3 (may also work on Python 2, but this is untested) that makes it easy to retrieve data from the Mortar API.
PyMortar wraps the underlying `GRPC API  <https://git.sr.ht/~gabe/mortar/tree/master/proto/mortar.proto>`_ in a simple package.
The Mortar API delivers timeseries data using a streaming interface; PyMortar buffers this data in memory before assembling the final Pandas DataFrame. **PyMortar will happily fill up your computer's memory if you request too much data**; you will need to be careful!

Installation
============

PyMortar is available on pip:

.. code-block:: bash

    pip install pymortar>=1.0.0

It depends on pandas and some GRPC-related packages. These should be installed automatically. You may find it helpful to install PyMortar inside a `virtual environment <https://docs.python.org/3/library/venv.html>`_ or use the `PyMortar Docker container <https://mortardata.org/docs/quick-start/>`_.

.. toctree::
   :maxdepth: 2
   :caption: Contents:



Indices and tables
==================

* :ref:`genindex`
* :ref:`modindex`
* :ref:`search`
