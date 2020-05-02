This is an example social media backend: see how this is generated from scratch.

## Generated files in this example

- graphql_models/\*
- models/\*
- helpers/\*
- resolver.go

TODO: make Youtube video for this example

## Steps to make this project

1. Start MariaDB container with social network.sql  
   `docker-compose up -d`

2. Generate /models/\* files  
   `sqlboiler mysql --no-back-referencing -d`

   --no-back-referencing if you don't set it your program will crash with recursive converts!

3. Generate graphql schema based on sqlboiler files  
   `go run github.com/web-ridge/sqlboiler-graphql-schema --output=schema.graphql --skip-input-fields=userId --directives=isAuthenticated --pagination=no`
4. Generate /graphql_models/\* + resolver.go  
   `go run convert_plugin.go`

5. ...Let's go!  
   `go run server.go resolver.go`
