# Chirpy

A guided project from [boot.dev](https://boot.dev/).

## Description

Chirpy is a web server implemented in Go that mimics how would a micro-blogging platform work from a backend point of view.

Is it written from scratch without the use of frameworks, but for a library to handle JSON Web Tokens.

Being a learning project, Chirpy implements the basics of a micro-blogging platform to grasp the inner workings of an application of this nature and is not intended for real use. But it does cover many real-life backend aspects like the following:

- Endpoint routing
- Processing of JSON requests and responses
- Parsing of HTTP headers and URL parameters
- Persistent storage using a database
- Handling of secrets like database credentials and user passwords
- Authentication and authorization using JSON Web Tokens (JWT) and refresh tokens
- Use of webhooks

Note that Chirpy doesn't come with a client.

## Motivation

Chirpy was developed to practice the implementation of web server.

## API

### GET /admin/metrics

- Purpose: to show number of visitors
- Availability: everyone
- Request: plain GET request
- Response:
  - Format: an HTML document with the number of visitors since last reset
  - HTTP codes:
    - 200 when successful

### POST /admin/reset

- Purpose: resets the database and the number of visitors
- Availability: restricted to the machine in `dev` mode
- Request: plain POST request
- Response:
  - Format: empty body
  - HTTP codes:
    - 200 when the operation was successful
    - 401 when not in `dev` mode
    - 500 when it was impossible to perform the database operation

### GET /api/chirps

- Purpose: to serve the chirps stored in the database
- Availability: everyone
- Request:
  - Optional URL parameters:
    - `author_id`: the `id` of the user whose chirps we want to retrieve. Default is everyone's chirps
    - `sort`: accept keywords `asc` and `desc` to sort the chirps in ascending or descending order, respectively, by time of creation. Default is `asc`
- Response:
  - Format:
    - On success: an array of JSON objects with the following key-value pairs:
      - `id`: the UUID of the chirp
      - `created_at`: timestamp (UTC) at which the chirp was stored in the database
      - `updated_at`: timestamp (UTC) at which the chirp was updated in the database
      - `body`: the text of the chirp with "profane" words removed
      - `user_id`: the UUID of the author of the chirp
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 200 when the operation was successful
    - 400 when passed an invalid author ID
    - 500 when it was impossible to perform the database operation

### POST /api/chirps

- Purpose: to save a chirp in the database
- Availability: to registered users
- Request:
  - HTTP Header: `Authorization: Bearer Access_token` with a valid `Access_token`
  - JSON payload: a JSON object with a `body` key and a value that holds the text of the chirp that should be stored in the database
- Response:
  - Format:
    - On success: a JSON object with the following key-value pairs:
      - `id`: the UUID of the chirp
      - `created_at`: timestamp (UTC) at which the chirp was stored in the database
      - `updated_at`: timestamp (UTC) at which the chirp was updated in the database
      - `body`: the text of the chirp with "profane" words removed
      - `user_id`: the UUID of the author of the chirp
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 201 when the operation was successful
    - 400
      - When the bearer token doesn't follow the required format
      - When the JSON object request doesn't conform to the requirements
      - When the text of the chirp is over 140 characters long
    - 401 when the bearer token can't be validated
    - 500 when it was impossible to perform the database operation

### GET /api/chirps/{chirpID}

- Purpose: to get a chirp by its ID
- Availability: everyone
- Request:
  - URL: must specify a valid `chirpID`
- Response:
  - Format:
    - On success: a JSON object with the following key-value pairs:
      - `id`: the UUID of the chirp
      - `created_at`: timestamp (UTC) at which the chirp was stored in the database
      - `updated_at`: timestamp (UTC) at which the chirp was updated in the database
      - `body`: the text of the chirp with "profane" words removed
      - `user_id`: the UUID of the author of the chirp
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 200 when the operation was successful
    - 400 when the given chirp UUID is invalid or missing
    - 404 when the requested chirp doesn't exist

### DELETE /api/chirps/{chirpID}

- Purpose: to delete a chirp by its ID
- Availability: only to the author of the chirp
- Request:
  - URL: must specify a valid `chirpID`
  - HTTP Header: `Authorization: Bearer Access_token` with a valid `Access_token`
- Response:
  - Format:
    - On success: empty body
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 204 when the operation was successful
    - 400
      - When the given chirp UUID is invalid
      - When the bearer token doesn't follow the required format
    - 401 when the bearer token can't be validated
    - 403 when the user making the request doesn't own the chirp to delete
    - 404 when the chirp doesn't exist
    - 500 when it was impossible to perform the database operation

### GET /api/healthz

- Purpose: to check the server status
- Availability: everyone
- Request: plain GET request
- Response:
  - Format:
    - On success: plain text body with code 200
    - On failure: N/A
  - HTTP codes:
    - 200 when the operation was successful

### POST /api/login

- Purpose: to log in a registered user
- Availability: only to registered users
- Request:
  - JSON payload: a JSON object with two key-value pairs: `email` and `password`
- Response:
  - Format:
    - On success: a JSON object with the following key-value pairs:
      - `id`: the UUID of the user
      - `created_at`: timestamp (UTC) at which the user registered
      - `updated_at`: timestamp (UTC) at which the user information was updated in the database
      - `email`: the user email
      - `is_chirpy_red`: whether the user has upgraded (boolean)
      - `token`: the authorization token
      - `refresh_token`: the refresh token
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 200 when the operation was successful
    - 400 when the JSON object request doesn't conform to the requirements
    - 401 when the password is incorrect for the given email
    - 500 when it was impossible to perform the database operation

### POST /api/polka/webhooks

- Purpose: to upgrade a user to subscriber
- Availability: only to Polka service
- Request:
  - HTTP Header: `Authorization: ApiKey API_Key`
  - JSON payload: an object with the following key-value pairs:
    - `"event": "user.upgraded"`
    - `"data"`
      - `"user_id"`: the UUID of the user to upgrade
- Response:
  - Format:
    - On success:
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 204
      - When JSON key `event` doesn't contain the value `"user.upgraded"`
      - When the operation was successful
    - 400
      - When the JSON object request doesn't conform to the requirements
      - When the user UUID is invalid
    - 401 when the API key can't be validated
    - 404 when the user to be upgraded isn't registered
    - 500 when it was impossible to perform the database operation

### POST /api/refresh

- Purpose: to refresh an access token using a refresh token
- Availability: to registered users
- Request:
  - HTTP Header: `Authorization: Bearer Refresh_token` with a valid `Refresh_token`
- Response:
  - Format:
    - On success: a JSON object with the `token` key and the new authorization token as value
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 200 when the operation was successful
    - 400 when the bearer token doesn't follow the required format
    - 401 when the refresh token is expired or has been revoked
    - 500 when it was impossible to perform the database operation

### POST /api/revoke

- Purpose: to revoke a refresh token
- Availability: to registered users
- Request:
  - HTTP Header: `Authorization: Bearer Refresh_token` with a valid `Refresh_token`
- Response:
  - Format:
    - On success: empty body
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 204 when the operation was successful
    - 400 when the bearer token doesn't follow the required format
    - 500 when it was impossible to perform the database operation

### POST /api/users

- Purpose: to register a new user
- Availability: everyone
- Request:
  - JSON payload: a JSON object with two key-value pairs: `email` and `password`
- Response:
  - Format:
    - On success: a JSON object with the following key-value pairs:
      - `id`: the UUID of the user
      - `created_at`: timestamp (UTC) at which the user registered
      - `updated_at`: timestamp (UTC) at which the user information was updated in the database
      - `email`: the user email
      - `is_chirpy_red`: whether the user has upgraded (boolean)
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 201 when the operation was successful
    - 400 when the JSON object request doesn't conform to the requirements
    - 500
      - When it was impossible to hash the new password
      - When it was impossible to perform the database operation

### PUT /api/users

- Purpose: to update the email and password of a registered user
- Availability: to registered users
- Request:
  - HTTP Header: `Authorization: Bearer Access_token` with a valid `Access_token`
  - JSON payload: a JSON object with two key-value pairs: `email` and `password`
- Response:
  - Format:
    - On success:
    - On failure: a JSON object with the `error` key and a message
  - HTTP codes:
    - 201 when the operation was successful
    - 400 when the JSON object request doesn't conform to the requirements
    - 401 when the bearer token can't be validated
    - 500
      - When it was impossible to hash the new password
      - When it was impossible to perform the database operation

## Running the app

### Configuration

The app reads the configuration file located at `~/.env` to work. This file must contain the following fields in pairs `key='value'`:

- `DB_URL`: a working connection string to a local PostgreSQL instance.
- `JWT_SECRET`: the secret string used to sign and validate JSON Web Tokens
- `POLKA_KEY`: the API key used to validate the origin of webhooks
- Optional `platform`: set to `'dev'` for testing the server

The connection string to the PostgreSQL database must have the following form:

```
'postgres://postgres:password@localhost:5432/chirpy?sslmode=disable'
```

The quotes are mandatory. We should replace `password` with the password we gave to the `postgres` user.

Also, note that `5432` is the port that PostgreSQL listens to by default. We must change that value in case our PostgreSQL instance is listening to a non-default port.

Finally, we specify `?sslmode=disable` to tell the app it shouldn't use SSL locally.

### Database migration

To migrate the `chirpy` database we created before, we should run the following command in the root directory of the project replacing the connection string with the one specified in the section before:

```bash
GOOSE_MIGRATION_DIR=sql/schema goose postgres "postgres://postgres:password@localhost:5432/chirpy" up
```

## Requirements

- A CPU architecture and operating system compatible with the Go runtime.
- Goose to run the database migrations.
- A working installation of PostgreSQL.
- Optionally, the Go compiler to install Goose with `go install`.

### Installing Goose on Linux

Goose is written in Go, so it's trivial to install it when we have Go available on our system:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

In case we don't have and don't want to install Go itself, we can install Goose running the [following command on Linux](https://pressly.github.io/goose/installation/#linux):

```bash
curl -fsSL \
    https://raw.githubusercontent.com/pressly/goose/master/install.sh |\
    sh
```

### Installing PostgreSQL on Ubuntu Linux

We install PostgreSQL on Ubuntu running the following commands:

```bash
# update the local repository cache
sudo apt update                    # alternative: aptitude update
# update the installed packages
sudo apt upgrade                   # alternative: aptitude safe-upgrade
# install PostgreSQL
sudo apt install postgresql postgresql-contrib  # alternative: aptitude install . . .
```

We then verify that PostgreSQL was installed successfully running `psql`, the official PostgreSQL client. This client provides the user with a shell where they can run SQL commands:

```bash
# verify that PostgreSQL was installed successfully
psql --version
```

### Starting the PostgreSQL server on Ubuntu Linux

After installation, we must run the following command to start the server:

```bash
sudo service postgresql start
```

We do this once on Ubuntu, just after installing PostgreSQL and before any reboot. After a reboot, the server should start automatically. This behavior may be different on other Linux distributions.

### Log in to the PostgreSQL server through `psql` in Ubuntu Linux

On testing environments, we can log in to the local PostgreSQL server using the PostgreSQL admin user:

```bash
# run psql as the postgres user
sudo -u postgres psql
```

In case that doesn't work, we can add the following rule to `pg_hba.conf`, the configuration file of PostgreSQL:

```postgres
local all postgres peer
```

That file is usually located at `/etc/postgresql/<major version>/main/pg_hba.conf`.

We now set a password for the `postgres` user running the following query on `psql`, replacing `<new password>` with the actual password:

```sql
ALTER USER postgres PASSWORD '<new password>';
```

Notice that, with that instruction, we're changing the password for the `postgres` database user and not the Linux user.

Finally, we create the `chirpy` database running the following command in `psql`:

```sql
CREATE DATABASE chirpy;
```

We close the `psql` shell typing `exit`.

## Keep developing the code

### Requirements

- `goose` to manage database migrations.
- `sqlc` to compile SQL queries into Go code.
- A working installation of PostgreSQL.

### Install dependencies

#### Standalone programs

Install Goose and `sqlc` by running the following commands:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Here are the up-to-date installation commands in case anything goes wrong when running the previous ones:

- [How to install Goose](https://github.com/pressly/goose#install).
- [How to install `sqlc`](https://docs.sqlc.dev/en/latest/overview/install.html).

#### Libraries

Go will download and install any dependencies found in `go.mod` and `go.sum` automatically when trying to run the code with `go run . <command> [arguments]`, but we can install the required libraries explicitly by running `go mod download`.

### Migrate the database

Run the following command at the root directory of the repository to migrate the database to the state expected by `chirpy`:

```bash
GOOSE_MIGRATION_DIR=sql/schema goose postgres "postgres://postgres:password@localhost:5432/chirpy" up
```

Notice that you should replace `password` with the actual password of the `postgres` user and, eventually, also the port that PostgreSQL is listening to.

### Configure `sqlc`

The code ships with a working configuration file located at `./sqlc.yaml`.

Read `sqlc`'s [documentation](https://docs.sqlc.dev/en/latest/tutorials/getting-started-postgresql.html) to know more about how to configure that program.

### Generate Go code from SQL queries using `sqlc`

After setting up `sqlc` with `./sqlc.yaml`, we generate Go code from the queries located at `./sql/queries` running `sqlc generate` from the root directory of the repository.
