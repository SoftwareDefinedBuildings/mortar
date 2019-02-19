VueRangedatePicker.default.install(Vue)
Vue.use(VueRouter)

var model = {
  text: '',
  editor: null,
  network: null,
  tablerows: [],
  tableheaders: [],
}

var _unselectedcolor = {
    border: '#0D47A1',
    background: '#BBDEFB',
}

var _selectedcolor = {
    border: '#B71C1C',
    background: '#E57373',
}

var dashdata = new Vue({
    data: {
        search: '',
        network: null,
        selected: [],
        table: {
            tablerows: [],
            tableheaders: [],
        },
        sites: [],
        graph: {
            nodes: new vis.DataSet([]),
            edges: new vis.DataSet([]),
        },
    },
    methods: {
        rowsToClassGraph: function() {
            var self = this;
            var q = "SELECT ?class1 ?relship ?class2 WHERE { ?n1 ?relship ?n2 . ?n1 rdf:type ?class1 . ?n2 rdf:type ?class2 };"
            hod.query(q).then( res => {
                //self.addQuery(res);
                var nodes = []
                var edges = []
                self.graph.nodes.clear();
                self.graph.edges.clear();
                res.obj.rows.forEach(function(r) {
                    var n1 = r.uris[0];
                    var rel = r.uris[1];
                    var n2 = r.uris[2];
                    if (!nodes.find(n => {return n.id == n1.value})) {
                        nodes.push({id: n1.value, label:n1.value});
                        self.graph.nodes.add({id: n1.value, label: n1.value, color: _unselectedcolor});
                    }
                    if (!nodes.find(n => {return n.id == n2.value})) {
                        nodes.push({id: n2.value, label:n2.value});
                        self.graph.nodes.add({id: n2.value, label: n2.value, color: _unselectedcolor});
                    }
                    if (!edges.find(e => { return e.from === n1.value && e.to === n2.value && rel.value === e.label})) {
                        if (rel.value != "hasSite" && rel.value != "isSiteOf" && (rel.value != "type" && n2.value != "Class")) {
                            edges.push({from: n1.value, to: n2.value, label: rel.value, color: _unselectedcolor})
                            self.graph.edges.add({from: n1.value, to: n2.value, label: rel.value});
                        }
                    }
                });
            });
        },
        addQuery: function(res) {
            var tablerows = [];
            var tableheaders = [];
            var self = this;
            res.obj.rows.forEach(function(r, rowidx) {
                var newrow = {rowidx: rowidx}
                for (var idx in r.uris) {
                    newrow[res.obj.variable[idx]] = r.uris[idx];
                }
                tablerows.push(newrow);
            });
            res.obj.variable.forEach(function(v) {
                tableheaders.push({
                    text: v,
                    align: 'left',
                    sortable: true,
                    value: v,
                });
            });
            self.$set(self.table, "tablerows", tablerows)
            self.$set(self.table, "tableheaders", tableheaders)
            self.$set(self, "sites", []);
            self.colorGraphFromQuery(res);
        },
        colorGraphFromQuery: function(res) {
            var self = this;

            let selectednodes = {};
            res.obj.rows.forEach(function(r, rowidx) {
                for (let uri of r.uris) {
                    selectednodes[uri.value] = 1;
                }
            });
            self.graph.nodes.forEach(function(found) {
                if (selectednodes[found.id] !== 1) {
                    found.color = _unselectedcolor
                    self.graph.nodes.update(found);
                } else {
                    found.color = _selectedcolor;
                    self.graph.nodes.update(found);
                }
            });
        },
        displayQualify: function(res) {
            var tablerows = [];
            var tableheaders = [];
            var self = this;
            tableheaders.push({
                text: "site",
                align: "left",
                sortable: true,
                value: "name",
            });
            tableheaders.push({
                text: "graph",
                align: "left",
                sortable: true,
                value: "name",
            });
            res.obj.sites.forEach(function(sitename, rowidx) {
                tablerows.push({
                    name: sitename,
                    rowidx: rowidx,
                });
            });
            self.$set(self.table, "tablerows", tablerows)
            self.$set(self.table, "tableheaders", tableheaders)
            self.$set(self, "sites", res.obj.sites);
            //this.$set(this.table, "tablerows", []);
            //this.$set(this.table, "tableheaders", []);
        },
        highlight: function(sitename) {
            var q = model.editor.getValue();
            var q2 = q.replace("WHERE", "FROM " + sitename + " WHERE");
            var self = this;
            hod.query(q2).then( res => self.addQuery(res));
        },
    },
})

var cache = new Vue({
    created: function(){
        localforage.setDriver(localforage.INDEXEDDB);
    },
    methods: {
        clear: function() {
            localforage.clear();
        },
        get: function(key) {
            return localforage.getItem(key);
        },
        put: function(key, value) {
            localforage.setItem(key, value);
        },
    },
})

var mdal = new Vue({
    methods: {
        query: function(q) {
            var self = this;
            return self.client.then( (res) => { return res.apis.MDAL.DataQuery({body: q}) })
        }
    },
    created: function() {
        var self = this;
        this.client = new SwaggerClient("/mdal.swagger.json")
    },
})

var mortar = new Vue({
    methods: {
        qualify: function(q) {
            var self = this;
            return self.client.then( (res) => { return res.apis.Mortar.Qualify({body: q}) })
        },
        testqualify: function() {
            var self = this;
            self.qualify({
                requiredqueries: [
                    { query: "SELECT ?x WHERE { ?x rdf:type brick:HVAC_Zone };" },
                ],
            }).then( res => {
                console.log("MORTAR",res);
            });
        },
    },
    created: function() {
        var self = this;
        this.client = new SwaggerClient("/mortar.swagger.json")
    },
})


var hod = new Vue({
    data:  {
        client: null,
        sites: [],
        sitemeta: {},
    },
    methods: {
        query: function(q) {
            var self = this;
            return cache.get(q).then( (res) => {
                if (res == undefined) {
                    return self.client.then( 
                                      (res) => { 
                                        return res.apis.HodDB.ExecuteQuery({body: {query: q}}) 
                                      }).then(
                                      (queryresult) => {
                                        return new Promise(function(resolve, reject) { cache.put(q, queryresult); return resolve(queryresult) });
                                      });
                }
                return new Promise(function(resolve, reject) { cache.put(q, res); return resolve(res) });
            });
        },
    },
    computed: {
        percentcomplete: function() {
            return 100 * Object.keys(this.sitemeta).length / this.sites.length;
        },
    },
    created: function() {
        this.client = new SwaggerClient("/hod.swagger.json");
    },
//    created: function() {
//        var self = this;
//        this.client = new SwaggerClient("/hod.swagger.json");
//
//        this.client.then(function(c) {
//            // get all sites
//            self.query("LIST NAMES;").then(function(res) {
//                res.obj.rows.forEach(function(r) {
//                    self.sites.push(r.uris[0].value);
//                    return r.uris[0].value;
//                });
//            }).then(function() {
//
//                // get stats for sites?
//                self.sites.forEach(function(site) {
//                    self.query("COUNT * FROM " + site + " WHERE { ?x rdf:type ?y };").then((res) => {
//                        //self.$set(self.sitemeta, site, {'entities': res.obj.count})
//                        //cache.put("COUNT * FROM " + site + " WHERE { ?x rdf:type ?y };", res);
//                        var meta = {'entities': res.obj.count};
//                        return meta;
//                    }).then( (meta) => {
//                        self.query("LIST VERSIONS FOR " + site + ";").then( (res) => {
//                            //cache.put("LIST VERSIONS FOR " + site + ";", res);
//                            if (res.obj.rows != null) {
//                                meta['versions'] = res.obj.rows.map(function(r) {
//                                    return r.uris[1].value;
//                                });
//                                meta['numversions'] = meta['versions'].length;
//                            }
//                            return meta;
//                        }).then( (r) => {
//                            self.query("SELECT ?prop ?value FROM " + site + " WHERE { ?s rdf:type brick:Site . ?s ?prop ?value };").then( (res) => {
//                                //cache.put("SELECT ?prop ?value FROM " + site + " WHERE { ?s rdf:type brick:Site . ?s ?prop ?value };", res);
//                                if (res.obj.rows != null) {
//                                    res.obj.rows.forEach(function(r) {
//                                        if ((r.uris[0].value != 'isSiteOf')) {
//                                            meta[r.uris[0].value] = r.uris[1].value;
//                                        }
//                                    });
//                                }
//                                self.$set(self.sitemeta, site, meta);
//                            });
//                            self.$set(self.sitemeta, site, meta);
//                        })
//                    });
//                });
//
//            }).then(function() {
//                self.sites.forEach(function(site) {
//                });
//            });
//
//        });
//    },
})

/*
 *
 */
const BrickBrowse = {
    created: function() {
      var scope = this;
      this.$nextTick(function() {
          console.log("CREATING EDITOR")
          model.editor = CodeMirror.fromTextArea(document.getElementById('editor'), {
            mode:  "application/sparql-query",
            matchBrackets: true,
            lineNumbers: true
          });
          
          //TODO: add a button to trigger the query
          //model.editor.on('change', function(cm) {
          //  console.log('changed', cm.getValue());
          //  model.text = cm.getValue();
          //  hod.query(cm.getValue()).then( res => {
          //      dashdata.addQuery(res);
          //  });
          //});
          model.editor.refresh()
          hod.query(model.editor.getValue()).then( res => {
              dashdata.addQuery(res);
              dashdata.rowsToClassGraph();
              var container = document.getElementById('mygraph');
		  	
              var data = {
                   nodes: dashdata.graph.nodes,
                   edges: dashdata.graph.edges
              };
              var options = {};
              model.network = new vis.Network(container, data, options);
          });



      });
    },
    methods: {
        runQuery: function() {
            hod.query(model.editor.getValue()).then( res => {
                dashdata.addQuery(res);
            });
        },
        runQualify: function() {
            mortar.qualify({
                requiredqueries: [{query: model.editor.getValue()}]
            }).then( res => {
                dashdata.displayQualify(res);
            });
        },
        filter: function(rows, term, x) {
            var terms = term.split(" ");
            return rows.filter( o => {
                var ret = false;
                for (let term of terms) {
                    ret = JSON.stringify(o).toLowerCase().indexOf(term) !== -1;
                    if (!ret) {
                        break
                    }
                }
                return ret
                
            });
        },
        sort: function(items, index, descending) {
            return items.sort(function(a, b) {
                if (index == null) {
                    return a.rowidx < b.rowidx;
                }
                if (descending) {
                    return a[index].value > b[index].value;
                } else {
                    return a[index].value < b[index].value;
                }
            })
        },
    },
    computed: {
        pageitems: function() {
            // use this to display 20 items by default
            return [13, {"text":"all","value":-1}]
        }
    },
    template: '\
        <div>\
                <v-layout row wrap>\
                    <v-flex md6 xs12>\
                        <v-card height="100%">\
                          <v-card-text><div id="mygraph"></div></v-card-text>\
                        </v-card>\
                    </v-flex>\
                    <v-flex md6 xs12>\
                        <v-layout row wrap>\
                            <v-flex xs10>\
                                <v-card height="30%">\
                                    <textarea id="editor" name="editor">SELECT ?equip ?equiptype ?point ?pointtype WHERE {\n\t?equip bf:hasPoint ?point .\n\t?equip rdf:type ?equiptype .\n\t?point rdf:type ?pointtype\n};</textarea>\
                                </v-card>\
                            </v-flex>\
                            <v-flex xs2>\
                                <v-btn color="success" v-on:click="this.runQuery">Run Query</v-btn>\
                                <v-btn color="info" v-on:click="this.runQualify">Qualify</v-btn>\
                            </v-flex>\
                            <v-flex xs12>\
                                <v-card v-if="dashdata.sites.length == 0" height="70%">\
                                  <v-card-title>\
                                      <v-text-field v-model="dashdata.search" append-icon="search" label="Search" single-line hide-details></v-text-field>\
                                  </v-card-title>\
                                  <v-card-text>\
                                    <v-data-table v-model="dashdata.selected" :custom-filter="filter" :custom-sort="sort" :search="dashdata.search" :headers="dashdata.table.tableheaders" :items="dashdata.table.tablerows" item-key="rowidx" select-all lass="elevation-1" :rows-per-page-items="this.pageitems">\
                                        <template slot="items" slot-scope="props">\
                                            <td><v-checkbox v-model="props.selected" primary hide-details></v-checkbox></td>\
                                            <td v-for="header in dashdata.table.tableheaders">{{ props.item[header.value].value }}</td>\
                                        </template>\
                                    </v-data-table>\
                                  </v-card-text>\
                                </v-card>\
                                <v-card v-else height="70%">\
                                    <v-card-title>\
                                        <h3>{{ dashdata.sites.length }} sites qualified</h3>\
                                    </v-card-title>\
                                    <v-card-text>\
                                        <v-data-table :headers="dashdata.table.tableheaders" :items="dashdata.table.tablerows" item-key="rowidx" lass="elevation-1" :rows-per-page-items="this.pageitems">\
                                            <template slot="items" slot-scope="props">\
                                                <td> {{ props.item.name }}</td>\
                                                <td> <v-btn v-on:click="dashdata.highlight(props.item.name)">Highlight</v-btn>\
                                            </template>\
                                        </v-data-table>\
                                    </v-card-text>\
                                </v-card>\
                            </v-flex>\
                        </v-layout>\
                    </v-flex>\
                </v-layout>\
        </div>\
    ',
}
const About = {
    template: '\
    <div>\
        <v-layout row wrap>\
            <v-flex md6 xs12 class="px-5">\
                <h4 class="text-xs-left headline">The <b>M</b>odular <b>O</b>pen <b>R</b>eproducibility <b>T</b>estbed for <b>A</b>nalysis and <b>R</b>esearch</h4>\
                <v-alert class="my-5" :value="true" type="warning">\
                    <b>Mortar is in Alpha!</b> Please <a href="https://lists.eecs.berkeley.edu/sympa/subscribe/mortar-users">subscribe</a> to the mortar-users mailing list for updates and announcements</a>\
                </v-alert>\
                <p>\
                    Access to large amounts of real-world data has long been a barrier to the development and evaluation of analytics applications for the built environment. Open data sets exist, but they are limited in their <u>span</u> (how much data is available) and <u>context</u> (what kind of data is available and how it is described).\
                </p>\
                <p>\
                    The goal of Mortar is to provide a large, diverse and consistently updated testbed of buildings and building data to facilitate reproducible evaluation of building analytics.\
                </p>\
                <p>\
                    At this time, Mortar contains <b>107 buildings</b>, spanning over <b>10 billion data points</b> and <b>26,000 data streams</b>. Context for these data streams  is provided by a <a href="https://brickschema.org/">Brick model</a>. The Brick model describes for each building: (1) what data streams exist and what they measure, (2) what equipment exists and how it is monitored, (3) the relationships between equipment in terms of flows, composition and location.\
                </p>\
                <p>\
                    <ul>\
                        <li><del><b><a href="https://auth.mortardata.org/login?response_type=code&client_id=39seicjl2i1setbu0ur075n1bp&redirect_uri=https://mortardata.org/login">Log In</a></b></del> Not yet!</li>\
                        <li><b><a href="https://auth.mortardata.org/signup?response_type=code&client_id=39seicjl2i1setbu0ur075n1bp&redirect_uri=https://mortardata.org/login">Sign Up</a></b></li>\
                        <li><b><a href="/docs/">Documentation</a></b></li>\
                        <li><b><a href="https://lists.eecs.berkeley.edu/sympa/subscribe/mortar-users">Subscribe</a></b> to the <a href="mailto:mortar-users@lists.eecs.berkeley.edu">mortar-users</a> mailing list for updates, announcements, discussions and support</li>\
                    </ul>\
                </p>\
            </v-flex>\
            <v-flex md6 xs12>\
                <img src="img/brickgraphsample.png"></img>\
            </v-flex>\
        </v-layout>\
    </div>\
    ',
}


const routes = [
  { path: '/', component: About , name: 'home'},
  { path: '/brick', component: BrickBrowse , name: 'brickbrowse'},
  { path: '/about', component: About , name: 'about'},
]

  // { path: '/equip/:site', component: EquipmentList, props: true, name: 'viewequip'},
const router = new VueRouter({
  routes // short for `routes: routes`
})


const app = new Vue({
  router,
  data: () => ({
    drawer: false,
  }),
  props: {
    source: String,
  },
  created: function() {
    console.log('vue mounted');
  },
  mounted: async function() {
    await mdal.clientcreated;
  },
}).$mount('#app')
