__version__ = '0.1.0'
from pymortar import mortar_pb2
from pymortar import mortar_pb2_grpc
from pymortar.result import Result

from pymortar.mortar_pb2 import GetAPIKeyRequest, FetchRequest, QualifyRequest, Stream, TimeParams, Timeseries, View, DataFrame

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

        mortar_address: address + port to connect to, e.g. "localhost:9001".
                        Defaults to $MORTAR_API_ADDRESS from the environment.
                        Currently expects a TLS-secured endpoint
        username: your Mortar API username. Defaults to MORTAR_API_USERNAME env var
        password: your Mortar API password. Defaults to MORTAR_API_PASSWORD env var

    Returns
    -------
    client: Client
        An instance of the Mortar Client.
    """
    def __init__(self, cfg=None):
        if cfg is not None:
            self._cfg = cfg
        else:
            self._cfg = {}

        # get username/password from environment or config file
        if 'username' not in self._cfg or not self._cfg['username']:
            self._cfg['username'] = os.environ.get('MORTAR_API_USERNAME')
        if 'password' not in self._cfg or not self._cfg['password']:
            self._cfg['password'] = os.environ.get('MORTAR_API_PASSWORD')

        if self._cfg.get('mortar_address') is None:
            self._mortar_address = os.environ.get('MORTAR_API_ADDRESS','mortardata.org:9001')
        else:
            self._mortar_address = self._cfg.get('mortar_address')

        # setup GRPC client: gzip + tls
        if self._cfg.get('abandon_all_tls') == "yes i'm sure":
            print('insecure')
            self._channel = grpc.insecure_channel(self._mortar_address, options=[
	        ('grpc.default_compression_algorithm', 2) # 2 is GZIP
	    ])
        else:
            credentials = grpc.ssl_channel_credentials()
            self._channel = grpc.secure_channel(self._mortar_address, credentials, options=[
	        ('grpc.default_compression_algorithm', 2) # 2 is GZIP
	    ])

        self._client = mortar_pb2_grpc.MortarStub(self._channel)

        # TODO: check if a .pymortartoken.json file exists. If it does, then use the token
        # and don't do the username/password login

        if os.path.exists(".pymortartoken.json"):
            self._token = json.load(open(".pymortartoken.json", "r"))
            #print("loaded token: {0}".format(self._token))
        else:
            self._token = None

        # TODO: handle the refresh token recycling automatically for the user
        # TODO: break this out into a method that can be called when we notice that the token is expired
        if self._token is None:
            self._refresh()

    def _refresh(self):
        response = self._client.GetAPIKey(mortar_pb2.GetAPIKeyRequest(username=self._cfg["username"],password=self._cfg["password"]))
        #print(response)
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
            #res.add(x)
            res.add2(x)
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
            if resp.error:
                raise Exception(resp.error)
            return resp
        except Exception as e:
            if hasattr(e, 'details') and e.details() == 'parse jwt token err: Token is expired':
                self._refresh()
                return self.qualify(required_queries)
            else:
                raise e
