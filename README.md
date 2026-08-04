# queryc

`queryc` generates type-safe Go bindings from SQL queries.

Write SQL and generate typed Go APIs, result structures, CRUD helpers, and prepared statements automatically.

Supports PostgreSQL and SQLite.

## Installation

```bash
go install github.com/AlexJarrah/queryc/cmd/queryc@latest
```

## Why queryc?

- Write plain SQL instead of query builders
- Generate typed bindings automatically
- Infer types directly from your schema
- Build complex nested and aggregated result structures
- Generate CRUD helpers and schema-safe SQL constants
- Automatically prepare static queries

---

# Quick Start

## Schema

```sql
CREATE TABLE users (
  user_id UUID PRIMARY KEY,
  email TEXT NOT NULL,
  name TEXT NOT NULL
);
```

---

## Query

```sql
@import({
  path: "example.com/app/models",
  alias: "models",
  schema: true
})

@query({ name: "GetUserByEmail" }) {
  SELECT * FROM users
  WHERE email = $email
  LIMIT 1;
}
```

---

## Generated Usage

Generated code lives in the `--package` flag's value (defaults to dialect name). The schema-marked `@import` supplies the application-owned row types and the alias used in generated code.

```go
query := postgres.GetUserByEmail("username@example.com")
user, err := query.GetOne(ctx, conn)
```

---

# CLI

Positional arguments:

- `schema`: SQL schema file or directory path, used for column type inference
- `queries`: queryc input file or directory
- `output`: generated bindings output file path

Flags:

- `--dialect`: SQL dialect, supports PostgreSQL & SQLite
- `--package`: generated Go package name (defaults to dialect name)

Examples:

```bash
queryc schema.sql queries.sql bindings.go --dialect sqlite
queryc schemas/ queries/ bindings.go --dialect postgres
queryc schema.sql queries.sql bindings.go --package db
```

---

# Core Concepts

## Queries

Queries are defined with the `@query` function, which accepts a metadata object (unquoted key JSON).

```sql
@query({
  name: "GetUserByEmail",
  description: "Retrieve a user by his/her email"
}) {
  SELECT *
  FROM users
  WHERE email = $email
  LIMIT 1;
}
```

---

### Metadata

| Key           | Required | Description                    |
| ------------- | -------- | ------------------------------ |
| `name`        | Yes      | Generated function name        |
| `description` | No       | GoDoc comment for the function |
| `deprecated`  | No       | Adds a deprecation notice      |

---

### Parameters

Parameters allow code to provide arguments to the SQL. Parameters are defined with the `$` prefix. Their types are automatically inferred via the SQL schema input, but can be explicitly overridden with the `$name:*string` syntax.

```sql
WHERE
  user_id = $user_id:uuid.UUID
  AND created_at >= $start
  AND created_at <= $end:sql.NullTime
  AND $search:*string IS NULL
LIMIT $limit:uint16
```

Queries with multiple parameters generate a params struct to be passed to the generated function:

```go
type GetUsersParams struct {
    UserID uuid.UUID
    Start  time.Time
    End    sql.NullTime
    Search *string
    Limit  uint16
}
```

---

## Dynamic SQL Fragments

SQL fragments allow raw SQL insertion into a query, allowing absolute customizability to a query. SQL fragments do not support queryc functions and support type casting in the same way parameters do.

Fragments become additional function arguments in generated code.

**Warning:** Fragments bypass SQL parameterization and are concatenated directly into the query. Only use them with trusted, allowlisted values (e.g. generated `UsersField` / `SortDirection` constants). Never pass untrusted user input as a fragment.

```sql
SELECT *
FROM users
WHERE #key:UsersField = $value
ORDER BY #key:UsersField #sort:SortDirection;
```

```go
func GetUsersByField(params GetUsersByFieldParams, key UsersField, sort SortDirection) *querycruntime.Query[...]
```

---

## Imports

The `@import` function allows you to specify Go imports for custom types, allowing for casting to external types such as sql.NullString or custom structs.

One import must be marked as the schema package when the input schema contains tables.

```sql
@import({
  path: "example.com/project/models",
  alias: "models",
  schema: true
})
```

---

## Structured Results

The `@struct({ ... })` function allows building structured outputs directly from SQL. You can target existing types or automatically generate new ones.

```sql
SELECT
  user_id,
  @struct({
    timezone: uc.timezone:*time.Location,
    email_notifications: uc.email_notifications
  }) AS configuration
FROM user_configurations;
```

```go
type Example struct {
    UserID uuid.UUID
    Configuration ExampleConfiguration
}

type ExampleConfiguration struct {
    Timezone           *time.Location
    EmailNotifications bool
}
```

---

## Slices

The `@slice` function builds slices from aggregated query results.

```sql
SELECT
  user_id,
  @slice((
    SELECT ul.login_at
    FROM user_logins ul
    WHERE ul.user_id = users.user_id
    ORDER BY ul.login_at DESC
  )) AS recent_logins,
  @slice((
    SELECT ua.alias
    FROM user_aliases ua
    WHERE ua.user_id = users.user_id
  )):[]string AS aliases
FROM users;
```

```go
type Example struct {
    UserID       uuid.UUID
    RecentLogins []ExampleRecentLogins
    Aliases      []string
}

type ExampleRecentLogins struct {
    LoginAt time.Time
}
```

`slice` also works with `struct`:

```sql
@slice(DISTINCT @struct({
  tag: ut.tag,
  source: ut.source
})) AS tags
```

---

# Generated

## Query Bindings

Queries generate a matching function with arguments for completion when applicable.

```sql
@query({
  name: "GetUserByEmail",
  description: "Retrieve a user by his/her email"
}) {
  SELECT
    user_id,
    dob AS birth_date,
    created_at AS join_date
  FROM users
  WHERE email = $email AND name = $name
  ORDER BY dob #sort:SortDirection;
}
```

```go
type GetUserByEmailParams struct {
    Email string
    Name  string
}

// Retrieve a user by his/her email
func GetUserByEmail(params GetUserByEmailParams, sort SortDirection) *Query[GetUserByEmailResult] { ... }

type GetUserByEmailResult struct {
    UserID    uuid.UUID
    BirthDate time.Time
    JoinDate  time.Time
}
```

---

## CRUD Functions

CRUD functions are automatically generated for every table in the schema. They
reference row types through the alias of the schema import.

Add, Get, Update, Delete, and Set functions use the table's primary key(s) as the arguments to the function. Inputs are validated (i.e. Add must have primary key, Update `row` must not).

```go
AddUser(row *schemas.User)
AddManyUsers(rows []schemas.User)
GetUser(user_id uuid.UUID)
GetAllUsers()
UpdateUser(user_id uuid.UUID, row *schemas.User)
DeleteUser(user_id uuid.UUID)
SetUser(row *schemas.User) // Create or update
```

---

## Schema-based Types & Constants

Types and constants are generated for safety with SQL fragments.

This includes:

- Constants for each table (i.e. `const Users Table = "users"`)
- Types for each table's columns (i.e. `type UsersField string`)
- Constants for each table column (i.e. `const UsersFieldUserID UsersField = "user_id"`)
- `SortDirection` type, containing `ASC` & `DESC` values

---

## Type Inference & Nullability

Types and nullability are inferred via:

- Referencing schema columns
- SQL casts
- Aggregate functions
- Derived tables and CTEs

Nullability is indicated by pointers; this can be overridden via explicit type casting or `COALESCE`.

---

## Embedded Tables

Selecting full tables embeds your schema package's row struct directly into the result.

```sql
SELECT users.*, user_configurations.*
FROM users u
LEFT JOIN user_configurations uc ON uc.user_id = u.user_id;
```

```go
type GetUsersWithConfiguration struct {
  User              schemas.User               `db:"User" table:"users"`
  UserConfiguration *schemas.UserConfiguration `db:"UserConfiguration" table:"user_configurations"`
}
```

---

## Scalar Fields

Named expressions become ordinary fields.

```sql
SELECT
  u.name AS user_name,
  COUNT(*)::bigint AS login_count
FROM users u;
```

```go
type ExampleResult struct {
    UserName   string
    LoginCount int64
}
```

---

## Prepared Statements

For PostgreSQL, static queries (no SQL fragments) can be prepared on a `*pgx.Conn` by calling `PrepareStatements` after connecting.

Prepared statements are local to the database connection, use pgx's automatic statement cache over `pgxpool.Pool` when querying via a pool.

---

# Full Example

```sql
@query({
  name: "GetUserProfile",
  description: "Retrieve all data related to a user"
}) {
  SELECT
    u.*,

    @struct({
      timezone: uc.timezone,
      theme: uc.theme
    }) AS settings,

    @slice((
      SELECT ut.tag
      FROM user_tags ut
      WHERE ut.user_id = u.user_id
    )):[]string AS tags

  FROM users u
  LEFT JOIN user_configurations uc ON uc.user_id = u.user_id
  WHERE u.user_id = $user_id
  ORDER BY u.name #direction:SortDirection
  LIMIT $limit:uint16;
}
```

```go
type GetUserProfileParams struct {
    UserID uuid.UUID
    Limit  uint16
}

// Retrieve all data related to a user
func GetUserProfile(params GetUserProfileParams, direction SortDirection) *Query[GetUserProfileResult] { ... }

type GetUserProfileResult struct {
    User      User
    Settings  GetUserProfileSettings
    Tags      []string
}

type GetUserProfileSettings struct {
    Timezone time.Location
    Theme    string
}
```
