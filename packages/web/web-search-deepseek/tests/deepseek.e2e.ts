import { describe, expect, it } from 'vitest'
import {
  DeepSeekSearchProvider,
  GLKJ_DEFAULT_API_VERSION,
  GLKJ_DEFAULT_BASE_URL,
  GLKJ_DEFAULT_MAX_TOKENS,
  GLKJ_DEFAULT_MAX_USES,
  GLKJ_DEFAULT_MODEL,
} from '@deepseek-ai/dsh-web-search-deepseek'

/** Construct the provider over a fixed options value; production passes a live thunk. */
import type { DeepSeekSearchProviderOptions } from '@deepseek-ai/dsh-web-search-deepseek'

const searchProvider = (options: DeepSeekSearchProviderOptions): DeepSeekSearchProvider =>
  new DeepSeekSearchProvider(() => options)

/**
 * Disabled real-API probe for the DeepSeek search provider. The live endpoint
 * can complete without structured source blocks, so this is not a reliable
 * merge signal. Its body remains because mocks cannot confirm the wire shape.
 */
const apiKey = process.env.GLKJ_API_KEY
const maybe = apiKey !== undefined && apiKey.length > 0 ? describe : describe.skip

maybe('DeepSeekSearchProvider real API', () => {
  it.skip('returns citeable sources for a live query via native web_search', async () => {
    const provider = searchProvider({
      apiKey: apiKey!,
      baseURL: process.env.GLKJ_SEARCH_BASE_URL ?? GLKJ_DEFAULT_BASE_URL,
      model: process.env.GLKJ_SEARCH_MODEL ?? GLKJ_DEFAULT_MODEL,
      apiVersion: GLKJ_DEFAULT_API_VERSION,
      maxTokens: GLKJ_DEFAULT_MAX_TOKENS,
      maxUses: GLKJ_DEFAULT_MAX_USES,
    })
    const result = await provider.search({ query: 'What is DeepSeek Harness?', maxResults: 5 })
    expect(result.sources.length).toBeGreaterThan(0)
    for (const source of result.sources) expect(source.url).toMatch(/^https?:\/\//)
  }, 60_000)
})
