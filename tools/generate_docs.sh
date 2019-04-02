#!/bin/bash
activate () {
  . docenv/bin/activate
}
activate
pip install --upgrade pymortar
cd pydocs
sphinx-apidoc -f -o source/ ../python/pymortar/pymortar/ 
make html

cd ..
mkdocs build
