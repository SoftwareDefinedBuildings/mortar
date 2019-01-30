FROM jupyter/scipy-notebook

COPY requirements.txt /tmp/
RUN pip install --requirement /tmp/requirements.txt && \
    fix-permissions $CONDA_DIR && \
    fix-permissions /home/$NB_USER

ADD mortar-client.key /certs/
ADD mortar-client.pem /certs/
ADD additional.sh /usr/local/bin/before-notebook.d/
ADD example.py /home/jovyan/
ADD start.sh /

CMD ["/start.sh"]
