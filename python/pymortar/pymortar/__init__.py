__version__ = '0.1.0'
from pymortar import mortar_pb2
from pymortar import mortar_pb2_grpc

from pymortar.mortar_pb2 import GetAPIKeyRequest, FetchRequest, Stream, TimeParams

from pymortar.mortar_pb2 import AGG_FUNC_RAW  as RAW
from pymortar.mortar_pb2 import AGG_FUNC_MEAN as MEAN
from pymortar.mortar_pb2 import AGG_FUNC_MIN as MIN
from pymortar.mortar_pb2 import AGG_FUNC_MAX as MAX
from pymortar.mortar_pb2 import AGG_FUNC_COUNT as COUNT
from pymortar.mortar_pb2 import AGG_FUNC_SUM as SUM

import pandas as pd

import os
import json
import grpc
import logging
logging.basicConfig(level=logging.DEBUG)

class Client:
    """
    Mortar client

    Parameters
    ----------
    cfg: dict
        Contains the configuration information for connecting to the Mortar API.
        Expects the following keys:

        mortar_address: address + port to connect to, e.g. "localhost:9001". Defaults to
                        looking up $MORTAR_API_ADDRESS from the environment.
        mortar_cert: relative file path to the mortardata api server certificate. If this is not
                     specified, then it defaults to an insecure connection.
                     **This is not recommended!**
        username: your Mortar API username
        password: your Mortar API password

    Returns
    -------
    client: Client
        An instance of the Mortar Client.
    """
    def __init__(self, cfg):
        self._cfg = cfg
        if self._cfg.get('mortar_address') is None:
            self._mortar_address = os.environ.get('MORTAR_API_ADDRESS','corbusier.cs.berkeley.edu:9001')
        else:
            self._mortar_address = self._cfg.get('mortar_address')

        # setup GRPC client
        # need options: insecure/secure (if mortar_cert provided),
        # gzip
        if self._cfg.get('mortar_cert') is None:
            # TODO: check https://github.com/grpc/grpc/blob/master/src/python/grpcio_tests/tests/unit/_compression_test.py
            self._channel = grpc.insecure_channel(self._mortar_address, options=[
                ('grpc.default_compression_algorithm', 2)
            ])
        else:
            # TODO: read trusted_certs from server.crt file
            credentials = grpc.ssl_channel_credentials(root_certificates=trusted_certs)
            self._channel = grpc.secure_channel(self._mortar_address, credentials, options=[
                ('grpc.default_compression_algorithm', 2) # 2 is GZIP
            ])

        self._client = mortar_pb2_grpc.MortarStub(self._channel)

        # TODO: check if a .pymortartoken.json file exists. If it does, then use the token
        # and don't do the username/password login

        if os.path.exists(".pymortartoken.json"):
            self._token = json.load(open(".pymortartoken.json", "r"))
            print("loaded token: {0}".format(self._token))
        else:
            self._token = None

        # TODO: handle the refresh token recycling automatically for the user
        # TODO: break this out into a method that can be called when we notice that the token is expired
        if self._token is None:
            response = self._client.GetAPIKey(mortar_pb2.GetAPIKeyRequest(username=self._cfg["username"],password=self._cfg["password"]))
            print(response)
            self._token = response.token
            json.dump(self._token, open(".pymortartoken.json", "w"))



    def fetch(self, request):
        """
        Calls the Mortar API Fetch command

        Parameters
        ----------
        req: mortar_pb2.FetchRequest
            TODO: need to document the fetch request parameters

            sites: list of strings
                Each string is a site name. These can be found through the qualify() API call

            streams: list of Streams
                Streams are how Mortar refers to collections of timeseries data.

            time: TimeParams
                Defines the temporal parameters for the data query

        Returns
        -------
        resp: pandas.DataFrame
            The column names are the UUIDs either explicitly annotated in the request or
            found through the Stream definitions

            TODO: figure out how to add in the metadata component
        """
        resp = self._client.Fetch(request, metadata=[('token', self._token)])
        builder = {}
        for x in resp:
            if x.error != "":
                logging.error(x.error)
                break
            if x.identifier not in builder:
                builder[x.identifier] = {'values': [], 'times': []}
            builder[x.identifier]['values'].extend(x.values)
            builder[x.identifier]['times'].extend(x.times)
        # copy the existing items because we are going to manipulate the dictionary
        # during the iteration
        items = list(builder.items())
        for uuidname, contents in items:
            # if its empty, then do not include the column in what is returned
            # TODO: why do we get empty uuids?
            if not uuidname:
                del builder[uuidname]
                continue
            ser = pd.Series(contents['values'], index=pd.to_datetime(contents['times']), name=uuidname)
            builder[uuidname] = ser[~ser.index.duplicated()]
        return pd.concat(builder.values(), axis=1)
