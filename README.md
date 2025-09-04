# Lost Dogs

This project is a Go application that monitors VK groups for posts about lost and found pets and sends them to a Telegram channel.

## Development Environment

This project uses Python and [Poetry](https://python-poetry.org/) to manage the Ansible version used for deployments. This ensures a reproducible deployment environment.

To install the dependencies, run:

```bash
poetry install
```

Then you can run Ansible commands with:

```bash
poetry run ansible-playbook ...
```

## Database Migrations

Database migrations are managed with `goose`. The migration files are located in the `resources/db/migrations` directory. There're migrations helpers in Justfile.
