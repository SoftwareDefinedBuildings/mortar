# PyMortar Documentation

PyMortar is a handy-dandy library for Python 3 (may also work on Python 2, but this is untested) that makes it easy to retrieve data from the Mortar API.
PyMortar wraps the underlying [GRPC API](https://git.sr.ht/~gabe/mortar/tree/master/proto/mortar.proto) in a simple package.
The Mortar API delivers timeseries data using a streaming interface; PyMortar buffers this data in memory before assembling the final Pandas DataFrame. *PyMortar will happily fill up your computer's memory if you request too much data*; you will need to be careful!

## Installation

PyMortar is available on pip:

```bash
$ pip install pymortar==0.2.7
```

It depends on pandas and some GRPC-related packages. These should be installed automatically. You may find it helpful to install PyMortar inside a [virtual environment](https://docs.python.org/3/library/venv.html) or use the [PyMortar Docker container](quick-start.md).

## Usage

### Creating a PyMortar Client

Best practices recommend that your Mortar API Username and Password (see [quick start](quick-start.md)) should be stored as environment variables `$MORTAR_API_USERNAME` and `$MORTAR_API_PASSWORD`, respectively.
You can also specify them as arguments to the PyMortar client constructor.

```python
import pymortar

# use environment variables
client = pymortar.Client()

# use explicit values
client = pymortar.Client({
   'username': 'my username',
   'password': 'my password'
})
```

### Mortar API: `Qualify`

The Mortar `Qualify` API call takes a list of Brick queries and returns the list of sites in the Mortar dataset that return results for all of the queries. This is helpful for determining which sites are worth investigating.

```python
response = client.qualify([
    # < Brick query 1 >,
    # < Brick query 2 >,
])
```

Example:

```python
meter_query = """SELECT ?meter 
WHERE { 
    ?meter rdf:type/rdfs:subClassOf* brick:Electric_Meter 
};"""

response = client.qualify([meter_query])
if resp.error != "":
    print("ERROR: ", resp.error)
    os.exit(1)
print("running on {0} sites".format(len(resp.sites)))
```

### Mortar API: `Fetch`

The Mortar `Fetch` API call takes as an argument a description of the timeseries data the client wants to download. This description is qualified by *metadata* in the form of Brick queries, and *temporally*.

A *timeseries stream* is the sequence of `<time, value>` pairs associated with a particular sensor, setpoint or other origin. A timeseries stream is identified by a *UUID*, which is a unique 36-byte identifier, e.g. `4daeab54-2517-11e9-ab30-54ee758a2ce3`. This corresponds to a column in a Pandas DataFrame.

A `pymortar.Stream` is a collection of timeseries streams that has been given a name and an aggregation. This collection can be specified explicitly as a list of UUIDs, or implicitly as a Brick query (recommended).

```python
# defining the meter stream explicitly
meter_stream = pymortar.Stream(
    name="meter",
    uuids=["4daeab54-2517-11e9-ab30-54ee758a2ce3", "15c0a642-2518-11e9-ab30-54ee758a2ce3"],
    aggregation=pymortar.MEAN,
)

# defining the meter stream implicitly
meter_query = """SELECT ?meter 
WHERE { 
    ?meter rdf:type/rdfs:subClassOf* brick:Electric_Meter 
};"""
meter_stream = pymortar.Stream(
    name="meter",
    definition=meter_query,
    dataVars=["?meter"],
    aggregation=pymortar.MEAN,
)
```

Available aggregation parameters are:

- `pymortar.MEAN`
- `pymortar.MAX`
- `pymortar.MIN`
- `pymortar.COUNT`
- `pymortar.SUM`
- `pymortar.RAW` (the temporal window parameter is ignored)

In the implicit case, we point out which variables in the query correspond to timeseries streams (here, it is just `?meter`). There is no need to add the `bf:uuid` relationship as in previous iterations of Mortar.

Temporal parameters have a start time and end time specified in the [RFC 3339 format](https://tools.ietf.org/html/rfc3339) (`2018-01-31T00:00:00Z`), and an aggregation window.
For each stream, Mortar returns all timeseries data between the start and end times, aggregated to the indicated frequency using the indicated aggregation function.

```python
# temporal parameters for the query: 2016-2018 @ 1hr aggregation
time_params = pymortar.TimeParams(
    start="2016-01-01T00:00:00Z",
    end="2018-01-01T00:00:00Z",
    window="1h",
)
```

Now all that's left is to put the query together and dispatch it to Mortar:

```python
meter_stream = pymortar.Stream(
    name="meter",
    definition=meter_query,
    dataVars=["?meter"],
    aggregation=pymortar.MEAN,
)

# temporal parameters for the query: 2016-2018 @ 1hr aggregation
time_params = pymortar.TimeParams(
    start="2016-01-01T00:00:00Z",
    end="2018-01-01T00:00:00Z",
    window="1h",
)

# form the full request object
request = pymortar.FetchRequest(
    sites=resp.sites, # from our call to Qualify
    streams=[meter_stream],
    time=time_params
)
response = client.fetch(request)
# Returns: <pymortar.result.Result: tables:1 cols:21 vals:8760>
```

### Working With Datasets

Once we have the response from the `Fetch` call (in the form of a `pymortar.Result` object), we can manipulate the returned metadata (`response.query`) and data (`response.df`).


#### Timeseries

`pymortar.Result` objects store all timeseries data for a `Fetch` call in a single Pandas DataFrame that is available under `response.df`:

```python
response.df.describe()
#       0ba942ac-9e44-3c0f-8529-8a59d3187a87  \
#       count                           8760.000000   
#       mean                            2662.821329   
#       std                              667.114554   
#       min                                0.000000   
#       25%                             2324.470078   
#       50%                             2592.125795   
#       75%                             3141.283526   
#       max                             3752.945826   
#   ...
```

The columns of the DataFrame are the timeseries stream UUIDs. UUIDs can be connected to their meaning and context through the Result object's metadata interface.

#### Metadata

`pymortar.Result` creates an in-memory SQLite table for each of the `pymortar.Stream`s in your `Fetch` query, containing the results of that query. The tables are named according to the name of the Stream, so it is simple to see the schema.

**TODO: add the API call that dumps the schema**

```python
response.query("SELECT sql FROM sqlite_master WHERE name = 'meter';")
# [(
#   'CREATE TABLE meter (
#     meter text, 
#     meter_uuid text, 
#     site text
#   )',
# )]
```

As you can see, the schema for the table consists of all of the variables from the Brick query's `SELECT` clause, the `_uuid` prefixed variable containing the UUID for that point, and the site that contains the Brick entity.

We can now manipulate the metadata using SQL! Some examples:

Finding out how many meters we have per site:

```python
response.query("SELECT site, COUNT(*) from meter GROUP BY site;")
#[('bwfp', 2),
# ('hwc', 2),
# ('lsa', 2),
# ('math', 2),
# ('rfs', 6),
# ('tupp', 2),
# ('vmep', 2),
# ...]
```

Looping through the meters for each site:

```python
for site in qualify_response.sites:
    rows = response.query("SELECT meter_uuid FROM meter WHERE site = '{0}';".format(site))
    uuids = [row[0] for row in rows] # flatten it out
    meter_dataframe = response.df[uuids] # pandas column selector
    print('total meter: ', meter_dataframe.sum(axis=1)) # sum all meters together
```
