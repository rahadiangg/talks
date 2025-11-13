FROM alpine/ansible:2.18.6

RUN rm -rf /usr/lib/python3.12/site-packages/ansible_collections/community/general && \
    ansible-galaxy collection install community.general:==11.4.0

WORKDIR /app

COPY *-playbook*.yaml ./
COPY execute-playbook.sh ./
COPY templates ./templates

CMD [ "sh", "execute-playbook.sh" ]