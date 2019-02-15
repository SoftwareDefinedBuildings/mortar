import pymortar
client = pymortar.Client({})

# client.qualify
qualify_resp = client.qualify([
    "SELECT ?meter WHERE { ?meter rdf:type/rdfs:subClassOf* brick:Building_Electric_Meter };",
])

req = pymortar.FetchRequest(
    sites=qualify_resp.sites,
    views=[
      pymortar.View(
        sites=qualify_resp.sites,
        name="meter",
        definition="SELECT ?meter WHERE { ?meter rdf:type/rdfs:subClassOf* brick:Building_Electric_Meter };",
      )
    ],
    dataFrames=[
      pymortar.DataFrame(
        name="meter_data",
        aggregation=pymortar.MEAN,
        window="1h",
        timeseries=[
          pymortar.Timeseries(
            view="meter",
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

resp = client.fetch(req)
print(resp)
