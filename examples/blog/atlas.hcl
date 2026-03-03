env "local" {
  src = "file://db/schema/schema.sql"
  dev = "docker://postgres/15/dev"
  migration {
    dir = "file://db/migrations"
  }
}
