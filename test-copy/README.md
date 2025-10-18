# COPY Command Test

This directory tests the COPY command functionality in dockstep.

## Test Contents

- `test-file.txt` - A simple text file to be copied
- `build.sh` - A shell script to be copied and executed
- `.dockerignore` - Specifies files to ignore during COPY
- `node_modules/` - Should be ignored by .dockerignore
- `temp/` - Should be ignored by .dockerignore

## Running the Test

```bash
cd test-copy
dockstep run copy-files
```

## Expected Behavior

1. The COPY command should copy all files except those in .dockerignore
2. The `node_modules/` and `temp/` directories should NOT be copied
3. The block should successfully execute the build.sh script
4. The output should show the copied files in /app

## Testing with Custom Context

See `../test-copy-custom-context/` for testing with a custom context directory specified via the `context` field in the block configuration.

