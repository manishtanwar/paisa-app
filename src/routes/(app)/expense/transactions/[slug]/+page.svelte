<script lang="ts">
  import COLORS from "$lib/colors";
  import { ajax, type Posting } from "$lib/utils";
  import _ from "lodash";
  import { onMount } from "svelte";
  import type { PageData } from "./$types";
  import PostingCard from "$lib/components/PostingCard.svelte";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import { iconify } from "$lib/icon";

  export let data: PageData;
  let postings: Posting[] = [];

  onMount(async () => {
    ({ postings } = await ajax("/api/account/transactions/:account", null, data));
    postings = _.chain(postings)
      .sortBy((p) => p.date)
      .reverse()
      .take(100)
      .value();
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns is-centered">
      <div class="column is-8">
        <div class="box py-2 mb-4 custom-icon is-size-5">
          <span>{iconify(data.account)}</span>
        </div>
        <ZeroState item={postings}>
          <strong>No transactions</strong> found for this account.
        </ZeroState>
        {#each postings as posting}
          <PostingCard
            {posting}
            color={posting.amount >= 0 ? COLORS.secondary : COLORS.tertiary}
          />
        {/each}
      </div>
    </div>
  </div>
</section>
