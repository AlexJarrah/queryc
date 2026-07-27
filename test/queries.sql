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
