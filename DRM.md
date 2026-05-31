# DRM Behavior Matrix

## INIT Handling Modes

### NO INIT MERGING

| # | Description                                                              | MPV | WEB |
|---|--------------------------------------------------------------------------|------|------|
| 1 | Init used to decrypt, then cleaned of PSSH/decryption info              | U    | U    |
| 2 | Init used to decrypt, original retained                                 | Y    | N    |

### INIT MERGING

| # | Description                                                              | MPV | WEB |
|---|--------------------------------------------------------------------------|------|------|
| 1 | Init merged with segment for decryption, then cleaned                   | U    | U    |
| 2 | Init merged with segment for decryption, original retained              | E    | E    |
