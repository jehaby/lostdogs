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
- Run a playbook (once added):
  poetry run ansible-playbook -i ansible/inventory.ini ansible/deploy.yml

Notes
- This repo pins `ansible-core` via Poetry and collections via `requirements.yml`.
- Prefer `ansible-core` over the meta `ansible` package for tighter control.
- Add/playbooks incrementally; start by mirroring the current deploy steps.

