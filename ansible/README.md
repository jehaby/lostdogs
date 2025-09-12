Ansible quickstart (via Poetry)

- Install Poetry (https://python-poetry.org/)
- Create the virtualenv and install pinned tools:
  poetry install
- Confirm versions:
  poetry run ansible --version
- Install required collections (once per environment):
  poetry run ansible-galaxy collection install -r ansible/requirements.yml

Usage

- Copy `ansible/inventory.example.ini` to `ansible/inventory.ini` and edit host/user.
- Create `ansible/group_vars/lostdogs.yml` from example and fill values:
  - cp ansible/group_vars/lostdogs.yml.example ansible/group_vars/lostdogs.yml
  - Edit vk_token, tg_token, tg_chat, image, etc.
- Build locally and deploy:
  - poetry run ansible-playbook ansible/deploy.yml \
     -e remote_path=/home/USER/infra/lostdogs \
     -e platform=linux/amd64

Details

- The playbook builds the Docker image locally, saves it to a tar, copies it to the remote,
  loads it, then runs `docker compose up` via `community.docker.docker_compose_v2` in `remote_path`.
- Override any value via inventory, group_vars, or `-e`.

Notes

- This repo pins `ansible-core` via Poetry and collections via `requirements.yml`.
- Prefer `ansible-core` over the meta `ansible` package for tighter control.
- Add/playbooks incrementally; start by mirroring the current deploy steps.
