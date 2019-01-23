import sqlite3
import pandas as pd

def format_uri(uri):
    if uri.namespace:
        return uri.namespace+"#"+uri.value
    else:
        return uri.value

def make_table(tablename, varnames):
    conn = sqlite3.connect(':memory:')
    c = conn.cursor()
    colnames = []
    for varname in varnames:
        varname = varname.lstrip('?')
        colnames.append( "{0} text".format(varname) )
    c.execute("CREATE TABLE {0} ({1}, site text)".format(tablename, ", ".join(colnames)))
    return conn

"""
The result object helps pymortar build from streaming responses to a query,
and provides an interface to look at both metadata and timeseries data that
is the output of a call to Fetch(...)
"""
class Result:
    def __init__(self):
        """
        Returns
        -------
        o: Result
            A Result object
        """

        self._table = {}
        self._series = {}
        self._df = None
        self._tables = {}

    def add(self, resp):
        """
        Adds the next FetchResponse object from the streaming call into
        the current Result object

        Parameters
        ----------
        resp: FetchResponse
            This parameter is a FetchResponse object obtained from
            calling the Mortar Fetch() call.
        """

        if resp.error != "":
            raise Exception(resp.error)


        #if resp.variable not in self._table and len(resp.variables) > 0:
        #    self._table[resp.variable] = make_table(resp.variable, resp.variables)
        #    self._tables[resp.variable] = list(map(lambda x: x.lstrip("?"), resp.variables))
        #    self._tables[resp.variable].append("site")

        #if resp.variable in self._table:
        #    c = self._table[resp.variable].cursor()
        #    for row in resp.rows:
        #        values = ['"{0}"'.format(format_uri(u)) for u in row.values]
        #        values.append('"{0}"'.format(resp.site))
        #        c.execute("INSERT INTO {0} values ({1})".format(resp.variable, ", ".join(values)))

        # SELECT * FROM sqlite_master;
        if resp.identifier:
            if resp.identifier not in self._series:
                self._series[resp.identifier] = []
            self._series[resp.identifier].append(
                pd.Series(resp.values, index=pd.to_datetime(resp.times), name=resp.identifier)
            )

    def build(self):
        for uuidname, contents in self._series.items():
            ser = pd.concat(contents)
            ser = ser[~ser.index.duplicated()]
            self._series[uuidname] = ser
        self._df = pd.concat(self._series.values(), axis=1, copy=False)

    @property
    def df(self):
        if self._df is None:
            self.build
        return self._df

    @property
    def tables(self):
        return list(self._tables.keys())

    def vars(self, table):
        return self._tables[table]

    def query(self, q):
        c = self._table.cursor()
        return list(c.execute(q))
