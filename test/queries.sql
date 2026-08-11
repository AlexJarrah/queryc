@import({
  path: "example.com/app/schemas",
  alias: "schemas",
  schema: true
})

@query({
  name: "GetUserWithConfiguration"
}) {
SELECT
  users.*,
  user_configurations.*,
  @struct({
    notifications_enabled: uc.email_notifications,
    timezone: uc.timezone
  }) AS preferences
FROM users
LEFT JOIN user_configurations uc ON uc.user_id = users.user_id
WHERE users.user_id = $user_id;
}

@query({
  name: "GetUserEmailWithLateral"
}) {
SELECT
  users.*,
  lu.email AS lateral_email
FROM users
LEFT JOIN LATERAL (
  SELECT u2.email
  FROM users u2
  WHERE u2.user_id = users.user_id
  LIMIT 1
) lu ON TRUE;
}
