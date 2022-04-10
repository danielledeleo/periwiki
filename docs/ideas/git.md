# git editing

## local
- a local daemon that runs a proxy git server that lets you edit pages of the wiki on your filesystem
- once committed, you would run `git push` as normal, pushing to `localhost:54321/user/periwiki.git`, where it actually submits edits to the remote wiki.
- No git repo exists on the server. Changes are translated locally into the Periwiki format.
- a "failed" push would be easier to reason about locally

## remote
- Similar to above, but periwiki runs the git server (associate SSH key with profile)

## in-browser
_(would require JavaScript)_
- a WebAssembly powered git client attached to the editor textbox (behaves like [local](#local))
- galaxy brain???