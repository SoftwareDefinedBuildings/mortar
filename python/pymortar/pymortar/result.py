import sqlite3
import time
import pandas as pd


def format_uri(uri):
    if uri.namespace:
        return uri.namespace+"#"+uri.value
    else:
        return uri.value

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
        self._selections = {}
        self._df = None
        self._dfs = {}
        self._tables = {}

    def __repr__(self):
        numtables = len(self._tables) if self._tables else "n/a"
        selections = self._selections.values()
        numcols = sum(map(lambda x: len(x.columns), self._dfs.values()))
        numvals = sum(map(lambda x: x.size, self._dfs.values()))
        values = [
            "collections:{0}".format(numtables),
            "selections:{0}".format(len(selections)),
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
        if resp.collection not in self._tables and len(resp.variables) > 0:
            make_table(self.conn, resp.collection, resp.variables)
            self._tables[resp.collection] = list(map(lambda x: x.lstrip("?"), resp.variables))
            self._tables[resp.collection].append("site")
        if resp.collection in self._tables:
            c = self.conn.cursor()
            for row in resp.rows:
                values = ['"{0}"'.format(format_uri(u)) for u in row.values]
                values.append('"{0}"'.format(resp.site))
                c.execute("INSERT INTO {0} values ({1})".format(resp.collection, ", ".join(values)))

        if resp.identifier and resp.selection:
            if resp.selection not in self._selections:
                self._selections[resp.selection] = {}
            if resp.identifier not in self._selections[resp.selection]:
                self._selections[resp.selection][resp.identifier] = []
            self._selections[resp.selection][resp.identifier].append(
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
        if len(self._selections) == 0:
            self._df = pd.DataFrame()
            return
        t = time.time()
        for selection, timeseries in self._selections.items():
            timeseries = self._selections[selection]
            for uuidname, contents in timeseries.items():
                ser = pd.concat(contents)
                ser = ser[~ser.index.duplicated()]
                self._selections[selection][uuidname] = ser
            self._dfs[selection] = pd.concat(self._selections[selection].values(), axis=1, copy=False)
        t2 = time.time()
        #print("Building DF took {0}".format(t2-t))

    def __getitem__(self, key):
        if key not in self._selections:
            return None
        if key not in self._dfs:
            self.build()
        return self._dfs[key]

    def __contains__(self, key):
        return key in self._selections

    def get(self, key, default=None):
        if key not in self._selections:
            return default
        return self[key]

    @property
    def collections(self):
        """
        Returns the list of collections in this result. Access collections as SQL
        tables using Result.query("select * from {collection name}")

        Returns
        -------
        l: list of str
          collection names
        """
        return self.tables

    @property
    def selections(self):
        """
        Returns the list of selections in this result. Access selections using
        Result['selection name'] or Result.get('selection name')

        Returns
        -------
        l: list of str
          selection names
        """
        return list(self._selections.keys())


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
