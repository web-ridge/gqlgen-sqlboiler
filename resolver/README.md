## Roadmap

- Automatically generate resolvers for every query (find input / filters) and write resolver
- Automatically generate resolvers for every mutation
- We group every resolver based on their model and naming in a folder and context
- Generate base resolver which calls all the generated resolvers

## Folder structure

Example of a folder structure

- resolver/user/users.go
- resolver/user/user.go
- resolver/user/create_user.go
- resolver/user/create_users.go
- resolver/user/delete_user.go
- resolver/user/update_user.go
- resolver/user/update_users.go

_Why?_  
Because we can easily resolve merge conflict if developer changed something or generator added something. Also we split by context since you're mostly working in one context and not the create of every model.
