#!/bin/sh

set -e

## Check environment varaiables before running the playbook

if [ -z "$LXD_HOST" ]; then
  echo "LXD_HOST environment variable is not set."
  exit 1
fi

if [ -z "$ANSIBLE_USER" ]; then
  echo "ANSIBLE_USER environment variable is not set."
  exit 1
fi

if [ -z "$ANSIBLE_PASSWORD" ]; then
  echo "ANSIBLE_PASSWORD environment variable is not set."
  exit 1
fi

if [ -z "$SUBDOMAIN" ]; then
  echo "SUBDOMAIN environment variable is not set."
  exit 1
fi

if [ -z "$EMAIL" ]; then
  echo "EMAIL environment variable is not set."
  exit 1
fi

if [ -z "$DB_ROOT_HOST" ]; then
  echo "DB_ROOT_HOST environment variable is not set."
  exit 1
fi

if [ -z "$DB_ROOT_USER" ]; then
  echo "DB_ROOT_USER environment variable is not set."
  exit 1
fi

if [ -z "$DB_ROOT_PASSWORD" ]; then
  echo "DB_ROOT_PASSWORD environment variable is not set."
  exit 1
fi

if [ -z "$WORDPRESS_THEME" ]; then
  echo "WORDPRESS_THEME environment variable is not set."
  exit 1
fi

if [ -z "$ACCESS_KEY" ]; then
  echo "ACCESS_KEY environment variable is not set."
  exit 1
fi

if [ -z "$SECRET_KEY" ]; then
  echo "SECRET_KEY environment variable is not set."
  exit 1
fi

if [ -z "$TOPIC_URN" ]; then
  echo "TOPIC_URN environment variable is not set."
  exit 1
fi

## Execute the Ansible playbook to create containers
ansible-playbook -i "$LXD_HOST," \
  -u "$ANSIBLE_USER" \
  -e "remote_tmp=/tmp/.ansible ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'" \
  -e "subdomain_name=$SUBDOMAIN ansible_password=$ANSIBLE_PASSWORD" \
  -e "container_name=web-$SUBDOMAIN" \
  01-playbook-create-containers.yaml

sleep 3

# Execute the Ansible playbook to configure reverse proxy
ansible-playbook -i ./inventory-reverse-proxy \
  02-playbook-configure-reverse-proxy.yaml

echo "Waiting for containers to be fully up..."
sleep 15

# Execute the Ansible playbook to setup WordPress
ansible-playbook -i ./inventory-container \
  --extra-vars "wordpress_admin_email=$EMAIL db_root_user=$DB_ROOT_USER db_root_password=$DB_ROOT_PASSWORD db_host=$DB_ROOT_HOST wordpress_theme=$WORDPRESS_THEME" \
  03-playbook-setup-wordpress.yaml

sleep 3

# send notification of successful deployment
# Read container info from file
if [ -f "./inventory-notification" ]; then
  . ./inventory-notification
else
  echo "Warning: inventory-notification file not found, using default port"
  CONTAINER_SSH_PORT="xxx"
fi

# If running on the LXD host
ansible-playbook -i "$LXD_HOST," \
  -u "$ANSIBLE_USER" \
  -e "remote_tmp=/tmp/.ansible ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'" \
  -e "ansible_password=$ANSIBLE_PASSWORD ansible_become_password=$ANSIBLE_PASSWORD" \
  -e "access_key=$ACCESS_KEY secret_key=$SECRET_KEY topic_urn=$TOPIC_URN" \
  -e "message_receiver=$EMAIL message_subject='Your website already live'" \
  -e "message_body='Congratulations! Your website is now live at <b>$SUBDOMAIN.onhuawei.cloud</b>. You can also login into SSH with port <b>$CONTAINER_SSH_PORT</b> and username/password <b>demohuawei</b>'" \
  04-playbook-smn-notification.yaml