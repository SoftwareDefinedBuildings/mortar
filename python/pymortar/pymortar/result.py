import sqlite3
import time
import pandas as pd


def format_uri(uri):
    if uri.namespace:
        return uri.namespace+"#"+uri.value
    else:
        return uri.value.strip('"')

def make_table(_conn, tablename, varnames):
    c = _conn.cursor()
    colnames = []
    for varname in varnames:
        varname = varname.lstrip('?')
        colnames.append( "{0} text".format(varname) )
    c.execute("CREATE TABLE {0} ({1}, site text)".format(tablename, ", ".join(colnames)))
    return _conn

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

        # result object has its own sqlite3 in-memory database
        self.conn = sqlite3.connect(':memory:')
        self._series = {}
        self._dataframes = {}
        self._df = None
        self._dfs = {}
        self._tables = {}

    def __repr__(self):
        numtables = len(self._tables) if self._tables else "n/a"
        dataframes = self._dataframes.values()
        numcols = sum(map(lambda x: len(x.columns), self._dfs.values()))
        numvals = sum(map(lambda x: x.size, self._dfs.values()))
        values = [
            "views:{0}".format(numtables),
            "dataframes:{0}".format(len(dataframes)),
            "timeseries:{0}".format(numcols),
            "vals:{0}".format(numvals)
        ]
        return "<pymortar.result.Result: {0}>".format(" ".join(values))

    def describe_table(self, tablename):
        """
        Prints out a description of the table with the provided name

        Parameters
        ----------
        tablename: string
            Table name. This will be from the pymortar.Stream object 'name' field. List can be retrieved using Result.tables()

        Returns
        -------
        n/a (prints out result)
        """
        s = "Columns: {0}".format(' '.join(self._tables.get(tablename, [])))
        s += "\nCount: {0}".format(self.query("SELECT COUNT(*) FROM {0}".format(tablename))[0][0])
        print(s)

    def add2(self, resp):
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
        if resp.view not in self._tables and len(resp.variables) > 0:
            make_table(self.conn, resp.view, resp.variables)
            self._tables[resp.view] = list(map(lambda x: x.lstrip("?"), resp.variables))
            self._tables[resp.view].append("site")
        if resp.view in self._tables:
            c = self.conn.cursor()
            for row in resp.rows:
                values = ['"{0}"'.format(format_uri(u)) for u in row.values]
                values.append('"{0}"'.format(resp.site))
                c.execute("INSERT INTO {0} values ({1})".format(resp.view, ", ".join(values)))

        if resp.identifier and resp.dataFrame:
            if resp.dataFrame not in self._dataframes:
                self._dataframes[resp.dataFrame] = {}
            if resp.identifier not in self._dataframes[resp.dataFrame]:
                self._dataframes[resp.dataFrame][resp.identifier] = []
            self._dataframes[resp.dataFrame][resp.identifier].append(
                pd.Series(resp.values, index=pd.to_datetime(resp.times), name=resp.identifier)
            )


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


        if resp.variable not in self._tables and len(resp.variables) > 0:
            make_table(self.conn, resp.variable, resp.variables)
            self._tables[resp.variable] = list(map(lambda x: x.lstrip("?"), resp.variables))
            self._tables[resp.variable].append("site")

        if resp.variable in self._tables:
            c = self.conn.cursor()
            for row in resp.rows:
                values = ['"{0}"'.format(format_uri(u)) for u in row.values]
                values.append('"{0}"'.format(resp.site))
                c.execute("INSERT INTO {0} values ({1})".format(resp.variable, ", ".join(values)))

        # SELECT * FROM sqlite_master;
        if resp.identifier:
            if resp.identifier not in self._series:
                self._series[resp.identifier] = []
            self._series[resp.identifier].append(
                pd.Series(resp.values, index=pd.to_datetime(resp.times), name=resp.identifier)
            )

    def build(self):
        if len(self._dataframes) == 0:
            self._df = pd.DataFrame()
            return
        t = time.time()
        for dataframe, timeseries in self._dataframes.items():
            timeseries = self._dataframes[dataframe]
            for uuidname, contents in timeseries.items():
                ser = pd.concat(contents)
                ser = ser[~ser.index.duplicated()]
                self._dataframes[dataframe][uuidname] = ser
            self._dfs[dataframe] = pd.concat(self._dataframes[dataframe].values(), axis=1, copy=False)
        t2 = time.time()
        #print("Building DF took {0}".format(t2-t))

    def __getitem__(self, key):
        if key not in self._dataframes:
            return None
        if key not in self._dfs:
            self.build()
        return self._dfs[key]

    def __contains__(self, key):
        return key in self._dataframes

    def get(self, key, default=None):
        if key not in self._dataframes:
            return default
        return self[key]

    @property
    def views(self):
        """
        Returns the list of views in this result. Access views as SQL
        tables using Result.query("select * from {view name}")

        Returns
        -------
        l: list of str
          view names
        """
        return self.tables

    @property
    def dataFrames(self):
        """
        Returns the list of dataframes in this result. Access dataframes using
        Result['dataframe name'] or Result.get('dataframe name')

        Returns
        -------
        l: list of str
          dataframe names
        """
        return list(self._dataframes.keys())


    @property
    def tables(self):
        """
        Returns a list of the table names, containing the retrieved metadata

        Returns
        -------
        l: list of string
            Each string is a table name
        """
        return list(self._tables.keys())

    def vars(self, table):
        """
        Returns a lsit of the column names for the given table

        Parameters
        ----------
        tablename: string
            Name of the table

        Returns
        -------
        l: list of string
            Column names of the table
        """
        return self._tables.get(table, [])

    def query(self, q):
        c = self.conn.cursor()
        return list(c.execute(q))
