__version__ = '0.1.0'
from pymortar import mortar_pb2
from pymortar import mortar_pb2_grpc
from pymortar.result import Result

from pymortar.mortar_pb2 import GetAPIKeyRequest, FetchRequest, QualifyRequest, Stream, TimeParams

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
            self._refresh()

    def _refresh(self):
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
        try:
            resp = self._client.Fetch(request, metadata=[('token', self._token)])
        except Exception as e:
            if e.details() == 'parse jwt token err: Token is expired':
                self._refresh()
                return self.fetch(request)
            else:
                raise e

        res = Result()
        for x in resp:
            if x.error != "":
                logging.error(x.error)
                break
            res.add(x)
        res.build()
        return res

    def qualify(self, required_queries):
        """
        Calls the Mortar API Qualify command

        Parameters
        ----------
        required_queries: list of str
            list of queries we want to use to filter sites

        Returns
        -------
        sites: list of str
            List of site names to be used in a subsequent fetch command
        """
        try:
            resp = self._client.Qualify(QualifyRequest(required=required_queries), metadata=[('token', self._token)])
            return resp
        except Exception as e:
            if e.details() == 'parse jwt token err: Token is expired':
                self._refresh()
                return self.qualify(required_queries)
            else:
                raise e
