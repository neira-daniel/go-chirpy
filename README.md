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
