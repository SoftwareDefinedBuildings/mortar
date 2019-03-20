import sqlite3
import time
import pandas as pd


def _format_uri(uri):
    if uri.namespace:
        return uri.namespace+"#"+uri.value
    else:
        return uri.value.strip('"')

def _make_table(_conn, tablename, varnames):
    c = _conn.cursor()
    colnames = []
    for varname in varnames:
        varname = varname.lstrip('?')
        colnames.append( "{0} text".format(varname) )
    c.execute("CREATE TABLE {0} ({1}, site text)".format(tablename, ", ".join(colnames)))
    return _conn

"""
"""
class Result:
    def __init__(self):
        """
        The result object helps pymortar build from streaming responses to a query,
        and provides an interface to look at both metadata and timeseries data that
        is the output of a call to Fetch(...)
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

    def describe_table(self, viewname):
        """
        Prints out a description of the table with the provided name

        Args:
            viewname (str): The name of the view you want to see a description of. A list of views
                can be retrieved using Result.views
        """
        s = "Columns: {0}".format(' '.join(self._tables.get(viewname, [])))
        s += "\nCount: {0}".format(self.query("SELECT COUNT(*) FROM {0}".format(viewname))[0][0])
        print(s)

    def view_columns(self, viewname):
        """
        Returns a Python list of strings corresponding to the column names of the given View

        Args:
            viewname (str): View name. This will be from the pymortar.View object 'name' field.
            List can be retrieved using the Result.views property

        Returns:
            columns (list of str): the column names for the indicated view
        """
        return self._tables.get(viewname, [])

    def view(self, viewname, fulluri=False):
        """
        Returns a pandas.DataFrame representation of the indicated view. This is presented
        as an alternative to writing SQL queries (Result.query)

        Args:
            viewname (str): View name. This will be from the pymortar.View object 'name' field.
                List can be retrieved using the Result.views property

        Keyword Args:
            fulluri (bool): (default: False) if True, returns the full URI of the Brick value.
                This can be cumbersome, so the default is to elide these prefixes

        Returns:
            df (pandas.DataFrame): a DataFrame containing the results of the View
        """
        cols = self.view_columns(viewname)
        col_str = ", ".join(cols)
        df = pd.DataFrame(self.query("select {0} from {1}".format(col_str, viewname)))
        df.columns = cols
        if not fulluri:
            for col in cols:
                df.loc[:, col] = df[col].str.split('#').apply(lambda x: x[-1])
        return df

    def _add(self, resp):
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
            _make_table(self.conn, resp.view, resp.variables)
            self._tables[resp.view] = list(map(lambda x: x.lstrip("?"), resp.variables))
            self._tables[resp.view].append("site")
        if resp.view in self._tables:
            c = self.conn.cursor()
            for row in resp.rows:
                values = ['"{0}"'.format(_format_uri(u)) for u in row.values]
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


    def _build(self):
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
            df = pd.concat(self._dataframes[dataframe].values(), axis=1, copy=False)
            # localize to UTC time
            self._dfs[dataframe] = df.set_index(df.index.tz_localize('UTC'))
        t2 = time.time()
        #print("Building DF took {0}".format(t2-t))

    def __getitem__(self, key):
        if key not in self._dataframes:
            return None
        if key not in self._dfs:
            self._build()
        return self._dfs[key]

    def __contains__(self, key):
        return key in self._dataframes

    def get(self, name, default=None):
        """
        Returns the DataFrame with the given name

        Args:
            name (str): name of the DataFrame from your FetchRequest

        Returns:
            df (pandas.DataFrame): DataFrame containing timeseries data
        """
        if key not in self._dataframes:
            return default
        return self[name]

    @property
    def views(self):
        """
        Returns the list of views in this result. Access views as SQL
        tables using `Result.query("select * from {view name}")`

        Returns:
            names (list of str): list of names of views (from FetchRequest)
        """
        return self.tables

    @property
    def dataFrames(self):
        """
        Returns the list of dataframes in this result. Access dataframes using
        Result['dataframe name'] or Result.get('dataframe name')

        Returns:
            names (list of str): list of names of dataframes (from FetchRequest)
        """
        return list(self._dataframes.keys())


    @property
    def tables(self):
        """
        Returns the list of views in this result. Access views as SQL
        tables using `Result.query("select * from {view name}")`

        Returns:
            names (list of str): list of names of views (from FetchRequest)
        """
        return list(self._tables.keys())

    def query(self, q):
        """
        Returns the result of a SQL query executed against the in-memory
        SQLite database of views returned by the query

        Args:
            query (str): SQL query.  Names of the tables are the names of the views

        Returns:
            results (list of []str): list of query results; each row of the result is a list
        """
        c = self.conn.cursor()
        return list(c.execute(q))
