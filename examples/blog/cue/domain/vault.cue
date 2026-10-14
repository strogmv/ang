package domain

import "github.com/strogmv/ang/cue/schema"

#UserVault: schema.#Entity & {
    id: string
    personalKey: string @encrypt(client_side="true")
    bio?: string @encrypt(client_side="true")
}
