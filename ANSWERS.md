## Why are you creating a unique ID when you get init and media segments in you dash manifest modifier?

Later, when I will need to get access to the init content to merge it repeatedly with media segments, I will need to know exactly which init fits which media, as that will allow me to merge them for decryption.
If I didn't do that, I would be relying on urls that may be IDENTICAL, but use $Representation$ or other placeholders, so I cannot base my IDs on the url, but on a specific generated ID.