import pymortar
client = pymortar.Client({})

# client.qualify
resp = client.qualify([
    "SELECT ?m WHERE { ?m rdf:type/rdfs:subClassOf* brick:Electric_Meter };",
])

req = pymortar.FetchRequest(
    sites=list(resp.sites)[:10],
    streams=[
        pymortar.Stream(
            name="meter",
            definition="SELECT ?meter WHERE { ?meter rdf:type/rdfs:subClassOf* brick:Electric_Meter };",
            dataVars=["?meter"],
            aggregation=pymortar.MEAN,
            units="",
        ),
    ],
    time=pymortar.TimeParams(
        start="2017-01-01T00:00:00Z",
        end="2018-01-01T00:00:00Z",
        window="1h",
    )
)

b = client.fetch(req)
print(b.df.describe())
