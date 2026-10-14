# Why I Built an AI-Friendly Compiler (And Why It Matters)

A few months ago, I was debugging generated code at 2 AM. Again. The AI had produced something that looked correct but wasn't. The types were wrong. The error handling was inconsistent. The imports were missing.

This wasn't the AI's fault. I was asking it to hold too much context in its head while writing Go code from scratch, every single time.

That's when I realized: the problem isn't AI capability. The problem is that we're using AI wrong.

## The Real Problem

Here's what happens when you ask an AI to write backend code:

1. You describe what you want
2. The AI writes code
3. You find bugs
4. You explain the bugs
5. The AI apologizes and writes more code
6. Repeat until context window exhausts

Sound familiar?

Each iteration burns context. Each correction requires explaining things the AI should already know. By the time you get working code, you've spent more tokens on corrections than on the actual feature.

I've been building server software for years, first at various companies, now as an architect working with AI-assisted development. And I kept seeing the same pattern: AI is brilliant at generating code, but terrible at maintaining consistency across a codebase.

## A Different Approach

What if instead of asking AI to write code, we asked it to declare intent?

That's ANG — Architectural Normalized Generator. It's a compiler that transforms declarative CUE definitions into production-ready Go code.

The philosophy is simple:
- **CUE defines the law** (what must exist)
- **Go executes the law** (how it works)
- **ANG enforces the law** (no deviation allowed)

When an AI writes CUE for ANG, it's writing a specification, not an implementation. The specification is small, structured, and verifiable. The implementation is generated — correctly, every time.

## What This Means for AI Context

Here's a real comparison.

**Without ANG**, to create a blog post API with authentication, ownership checks, and state machine transitions, an AI needs to:
- Understand your project structure
- Remember your naming conventions
- Know which packages you import
- Handle error patterns consistently
- Wire up dependencies correctly
- Write tests that actually test something

That's thousands of tokens of context before writing a single line of business logic.

**With ANG**, the AI writes this:

```cue
PublishPost: #Operation & {
    service: "blog"
    auth: true
    roles: ["admin"]

    input: { id: string }
    output: { ok: bool, publishedAt: string }

    flow: [
        {action: "repo.Find", source: "Post", input: "req.ID", output: "post"},
        {action: "logic.Check", condition: "post.Status == \"review\"", throw: "Only reviewed posts can be published"},
        {action: "fsm.Transition", entity: "post", to: "published"},
        {action: "repo.Save", source: "Post", input: "post"},
    ]
}
```

That's it. ANG generates the handler, the validation, the error handling, the OpenAPI spec, and the TypeScript client. All consistent with your architecture. All correct.

The AI doesn't need to remember your conventions. The conventions are encoded in the compiler.

## Guaranteed Quality

Here's what I mean by "guaranteed":

- **Type safety**: CUE validates your schema before generation
- **Consistent patterns**: Same input always produces same output
- **No missing imports**: The compiler knows what's needed
- **Proper error handling**: RFC 9457 errors, always
- **Working tests**: Contract tests generated from the spec

When the code compiles, it works. When it works, it works the same way everywhere.

I've seen teams spend weeks debugging subtle inconsistencies in AI-generated code. With ANG, those inconsistencies don't exist. Not because the AI is smarter, but because there's less room for error.

## Not Perfect, But Improving

I won't pretend ANG solves everything. It doesn't.

There are patterns it doesn't support yet. There are edge cases that require custom code. The documentation could be better. The error messages could be clearer.

But here's the thing: it's open source. If you find something that doesn't work for your use case, you can fix it. Or tell me about it, and we'll fix it together.

The project is at [github.com/strogmv/ang](https://github.com/strogmv/ang). There's a blog example that demonstrates most features: authentication, CRUD operations, state machines, events, and role-based access control.

## Why This Matters Now

AI coding assistants are getting better every month. But "better" means they can do more — which means more context, more complexity, more chances for inconsistency.

The solution isn't to fight this. It's to give AI the right level of abstraction. Let it work with intent, not implementation. Let the compiler handle the boring parts.

I built ANG because I was tired of debugging generated code at 2 AM. Now I debug CUE definitions at reasonable hours instead. It's a significant improvement.

If you're building Go backends with AI assistance, maybe it can help you too.

---

*ANG is free and open source. Install with: `go install github.com/strogmv/ang/cmd/ang@latest`*

*Questions? Feedback? Find me here or open an issue on GitHub.*
