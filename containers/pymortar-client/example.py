import pymortar
client = pymortar.Client({})

# client.qualify
resp = client.qualify([
    "SELECT ?m WHERE { ?m rdf:type/rdfs:subClassOf* brick:Electric_Meter };",
])

req = pymortar.FetchRequest(
    sites=resp.sites,
    collections=[
      pymortar.Collection(
        sites=resp.sites,
        name="meter",
        definition="SELECT ?meter WHERE { ?meter rdf:type/rdfs:subClassOf* brick:Building_Electric_Meter };",
      )
    ],
    selections=[
      pymortar.Selection(
        name="meter_data",
        aggregation=pymortar.MEAN,
        window="1h",
        timeseries=[
          pymortar.Timeseries(
            collection="meter",
            dataVars=["?meter"],
          )
        ],
      )
    ],
    time=pymortar.TimeParams(
        start="2017-01-01T00:00:00Z",
        end="2018-01-01T00:00:00Z",
    )
)

b = client.fetch(req)
print(b.df.describe())
