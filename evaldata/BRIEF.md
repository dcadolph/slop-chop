# What to send a rater

The rating study in [ANALYSIS.md](ANALYSIS.md) needs people who have never seen this
project. That rules out sending them a link to the repository, this directory, or the site.
Send them the text below in a message, with the spreadsheet attached, and nothing else. The
file comes from `go run ./evaldata/harness -rate-sheet sheet.csv -rate-pilot 20 -rate-seed N`,
with a different `N` for each person so no two of them read the passages in the same order.

Do not name the tool, the rules, or what the passages were chosen to test. A rater who
knows what is being looked for is rating the question instead of the text.

## The message

> I am running a small reading study and could use twenty minutes of your time.
>
> Attached is a spreadsheet with twenty short passages. Each one is a paragraph or two of
> documentation from a software project. For each passage, read it and answer one question
> in the last column: how machine-written does it read to you? Put a number from 1 to 7,
> where 1 means you are sure a person wrote it and 7 means you are sure a machine did. Use
> the middle numbers when you are not sure. There is no right answer and no trick.
>
> A few things that matter for the result:
>
> - Go with your read of the text. No searching for the passages, no tools, no asking.
> - Answer every row. A blank row cannot be used.
> - Do not change anything but the last column.
> - Do not discuss the passages with anyone else doing this until everyone is done.
>
> When you are finished, send the file back. Your answers are stored under an id that is
> not your name, and the anonymized answers will be published with the result, whatever it
> turns out to be.
>
> One question before you start: have you read or heard anything about a writing tool I
> have been working on this year? If so, tell me and skip the sheet. That is not a problem,
> it just means you are not the right reader for this one.

## Importing what comes back

```sh
go run ./evaldata/harness -rate-import filled.csv -rate r01
go run ./evaldata/harness
```

The second command reports agreement and reads no score. The pilot decision in
[ANALYSIS.md](ANALYSIS.md) is made from that number alone.
