binary := "hetzner-backup"

# Download deps and build the binary
install:
    go mod tidy
    go build -o {{binary}} .

# Build only
build:
    go build -o {{binary}} .

# Run without building a binary (needs secret.json in this dir, or pass a path)
run *args:
    go run . {{args}}

# Remove the built binary
clean:
    rm -f {{binary}}
