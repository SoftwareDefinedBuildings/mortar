# PyMortar Documentation

PyMortar is a handy-dandy library for Python 3 (may also work on Python 2, but this is untested) that makes it easy to retrieve data from the Mortar API.
PyMortar wraps the underlying [GRPC API](https://git.sr.ht/~gabe/mortar/tree/master/proto/mortar.proto) in a simple package.
The Mortar API delivers timeseries data using a streaming interface; PyMortar buffers this data in memory before assembling the final Pandas DataFrame. *PyMortar will happily fill up your computer's memory if you request too much data*; you will need to be careful!

## Installation

PyMortar is available on pip:

```bash
$ pip install pymortar==0.3.1
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

#### Collections

A `pymortar.Collection` captures the results of a Brick query as a named SQL table. The results can be used to specify timeseries streams whose data we want to retrieve. 

```proto
message Collection {
    // name of the collection
    string name = 1;
    // sites included in this collection
    repeated string sites = 2;
    // brick query definition
    string definition = 3;
}
```

Examples:

```python
# building meters
meter_collection = pymortar.Collection(
    name="meters",
    sites=qualify_response.sites,
    definition="SELECT ?meter WHERE { ?meter rdf:type/rdfs:subClassOf* brick:Building_Electric_Meter };",
)

# thermostat points
tstat_point_collection = pymortar.Collection(
    name="tstat_points",
    sites=qualify_response.sites,
    definition="""SELECT ?tstat ?state ?temp ?hsp ?csp
        WHERE {
            ?tstat rdf:type brick:Thermostat .

            ?tstat bf:hasPoint ?state .
            ?tstat bf:hasPoint ?temp .
            ?tstat bf:hasPoint ?hsp .
            ?tstat bf:hasPoint ?csp .

            ?state rdf:type/rdfs:subClassOf* brick:Thermostat_Status .
            ?temp  rdf:type/rdfs:subClassOf* brick:Temperature_Sensor  .
            ?hsp   rdf:type/rdfs:subClassOf* brick:Supply_Air_Temperature_Heating_Setpoint .
            ?csp   rdf:type/rdfs:subClassOf* brick:Supply_Air_Temperature_Cooling_Setpoint .
        };""",
)


# also includes temperature sensors from thermostats! If we want to separate the two
# groups, we can use SQL joins on the resulting tables
temperature_sensors = pymortar.Collection(
    name="temp_sensors",
    definition="""SELECT ?temp ?room ?zone
        WHERE {
            ?temp rdf:type/rdfs:subClassOf* brick:Temperature_Sensor .
            ?temp bf:isLocatedIn ?room .
            ?room rdf:type brick:Room .
            ?room bf:isPartOf ?zone .
            ?zone rdf:type brick:HVAC_Zone
        };""",
)
```

#### Selections

A `pymortar.Selection` is a set of timeseries streams that has been given a name, an aggregation and window size for the aggregation. The selection can be specified as an *early binding* using a list of UUIDs, or as a *late binding* in the form of references to variables in `Collections`.

```proto
message Selection {
    // name of the selection
    string name = 1;

    // aggregation function
    AggFunc aggregation = 2;
    // window argument for aggregation
    string window = 3;
    // engineering units
    string unit = 4;

    // refer to variables in collections
    repeated Timeseries timeseries = 5;
    // instead of vars in collections, list the UUIDs explicitly.
    repeated string uuids = 6;
}

message Timeseries {
    // name of the Collection
    string collection = 1;
    // list of variables from the collection that
    // we want to get data for
    repeated string dataVars = 2;
}
```

Examples:

```python
# defining the meter stream explicitly
meter_stream = pymortar.Selection(
    name="meter_data",
    uuids=["4daeab54-2517-11e9-ab30-54ee758a2ce3", "15c0a642-2518-11e9-ab30-54ee758a2ce3"],
    aggregation=pymortar.MEAN,
    window="15m"
)

# defining the meter stream implicitly using the 'meters' Collection above
meter_stream = pymortar.Selection(
    name="meter_data",
    aggregation=pymortar.MEAN,
    window="15m",
    timeseries=[
        pymortar.Timeseries(
            collection="meters",
            dataVars=["?meter"],
        )
    ]
)

# 15 min mean aggregation for all temperature sensors; from thermostats and from the broader
# query. This will have some overlap, but we can distinguish between the two groups later.
temperature_streams = pymortar.Selection(
    name="temperature_sensor_data",
    aggregation=pymortar.MEAN,
    window="15m",
    timeseries=[
        pymortar.Timeseries(
            collection="tstat_points",
            dataVars=["?temp"],
        ),
        pymortar.Timeseries(
            collection="temp_sensors",
            dataVars=["?temp"],
        ),
    ]
)

# for the integer-valued streams (thermostat state, heating/cooling setpoint),
# we want to snap the aggregation to a real value that the device experienced;
# we can't have a thermostat state of 2.5 for example.
thermostat_status_streams = pymortar.Selection(
    name="thermostat_status_data",
    aggregation=pymortar.MAX,
    window="1m",
    timeseries=[
        pymortar.Timeseries(
            collection="tstat_points",
            dataVars=["?state","?hsp","?csp"]
        )
    ]
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

Temporal parameters have a start time and end time specified in the [RFC 3339 format](https://tools.ietf.org/html/rfc3339) (`2018-01-31T00:00:00Z`).
For each stream, Mortar returns all timeseries data between the start and end times, aggregated to the indicated frequency using the indicated aggregation function.

```python
# temporal parameters for the query: 2016-2018 @ 1hr aggregation
time_params = pymortar.TimeParams(
    start="2016-01-01T00:00:00Z",
    end="2018-01-01T00:00:00Z",
)
```

Now all that's left is to put the query together and dispatch it to Mortar:

```python
# form the full request object
request = pymortar.FetchRequest(
    sites=qualify_response.sites, # from our call to Qualify
    collections=[
        meter_collection,
        tstat_point_collection,
        temperature_sensors
    ],
    selections=[
        meter_stream,
        temperature_streams,
        thermostat_status_streams
    ],
    time=time_params
)
result = client.fetch(request)
```

### Working With Datasets

Once we have the response from the `Fetch` call (in the form of a `pymortar.Result` object), we can manipulate the returned metadata (`result.collections`) and data (`result.selections`).


#### Timeseries

`pymortar.Result` objects store all timeseries data for a `Fetch` call in a different pandas DataFrame for each Selection.

```python
result.selections
# ['meter_data','temperature_sensor_data','thermostat_status_data']

result['meter_data'].describe()
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

Access the DataFrame containing the data for a Selection using the `result[<selection name>]` syntax, or the `result.get(<selection name>).
The names of all Selections can be obtained from the `result.selections` property.

The columns of each DataFrame are the timeseries stream UUIDs. UUIDs can be connected to their meaning and context through the Result object's metadata interface.

#### Metadata

`pymortar.Result` creates an in-memory SQLite table for each of the `pymortar.Collection`s in your `Fetch` query, containing the results of that query. The tables are named according to the name of the Collection, so it is simple to see the schema.

```python
result.describe_table("meters")
# columns: ?meter ?meter_uuid ?site
```

As you can see, the schema for the table consists of all of the variables from the Brick query's `SELECT` clause, the `_uuid` prefixed variable containing the UUID for that point, and the site that contains the Brick entity.

We can now manipulate the metadata using SQL! Some examples:

Finding out how many meters we have per site:

```python
result.query("SELECT site, COUNT(*) from meters GROUP BY site;")
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
    rows = result.query("SELECT meter_uuid FROM meters WHERE site = '{0}';".format(site))
    uuids = [row[0] for row in rows] # flatten it out
    meter_dataframe = result['meter_data'][uuids] # pandas column selector
    print('total meter: ', meter_dataframe.sum(axis=1)) # sum all meters together
```
