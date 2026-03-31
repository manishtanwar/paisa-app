<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import { fly } from "svelte/transition";
  import { formatCurrency, type Posting } from "$lib/utils";
  import dayjs from "dayjs";

  export let isOpen: boolean = false;
  export let postings: Posting[] = [];
  export let title: string = "Transactions";

  let searchQuery: string = "";
  let filteredPostings: Posting[] = [];

  const dispatch = createEventDispatcher();

  $: {
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      filteredPostings = postings.filter(
        (p) =>
          p.payee?.toLowerCase().includes(q) ||
          p.account?.toLowerCase().includes(q) ||
          p.narration?.toLowerCase().includes(q)
      );
    } else {
      filteredPostings = postings;
    }
  }

  function close() {
    isOpen = false;
    searchQuery = "";
    dispatch("close");
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      close();
    }
  }

  onMount(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  });
</script>

{#if isOpen}
  <!-- Backdrop -->
  <div
    class="drawer-backdrop"
    on:click={close}
    on:keydown={handleKeyDown}
    role="button"
    tabindex="0"
  ></div>

  <!-- Drawer -->
  <div class="drawer" transition:fly={{ x: 400, duration: 300 }}>
    <div class="drawer-header">
      <div class="drawer-title">
        <span class="icon">
          <i class="fas fa-list"></i>
        </span>
        <h3>{title}</h3>
      </div>
      <button class="delete is-medium" on:click={close} aria-label="Close"></button>
    </div>

    <div class="drawer-search">
      <div class="control has-icons-left">
        <input
          class="input"
          type="text"
          placeholder="Search by payee or account..."
          bind:value={searchQuery}
          autofocus
        />
        <span class="icon is-left">
          <i class="fas fa-search"></i>
        </span>
      </div>
    </div>

    <div class="drawer-content">
      {#if filteredPostings.length === 0}
        <div class="empty-state">
          <span class="icon is-large">
            <i class="fas fa-inbox fa-2x"></i>
          </span>
          <p>No transactions found</p>
        </div>
      {:else}
        <div class="transactions-count">
          Showing {filteredPostings.length} of {postings.length} transactions
        </div>
        <div class="transactions-list">
          {#each filteredPostings as p}
            <div class="transaction-item">
              <div class="transaction-header">
                <span class="transaction-date">
                  <i class="fas fa-calendar"></i>
                  {dayjs(p.date).format("DD MMM YYYY")}
                </span>
                <span class="transaction-amount" class:negative={p.amount < 0}>
                  {formatCurrency(p.amount)}
                </span>
              </div>
              <div class="transaction-payee">{p.payee || p.narration || ""}</div>
              <div class="transaction-account">{p.account}</div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style lang="scss">
  .drawer-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 999;
  }

  .drawer {
    position: fixed;
    top: 0;
    right: 0;
    width: 450px;
    max-width: 100vw;
    height: 100vh;
    background: #ffffff;
    box-shadow: -4px 0 24px rgba(0, 0, 0, 0.2);
    z-index: 1000;
    display: flex;
    flex-direction: column;
  }

  :global(.dark) .drawer {
    background: #1a1a2e;
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid #e0e0e0;
    background: #f5f5f5;
  }

  :global(.dark) .drawer-header {
    background: #252540;
    border-color: #3a3a5a;
  }

  .drawer-title {
    display: flex;
    align-items: center;
    gap: 0.75rem;

    .icon {
      color: var(--bulma-primary);
    }

    h3 {
      margin: 0;
      font-size: 1.1rem;
      font-weight: 600;
      color: var(--bulma-text-strong);
    }
  }

  .drawer-search {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--bulma-border-weak);
  }

  .drawer-content {
    flex: 1;
    overflow-y: auto;
    padding: 0;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 200px;
    gap: 1rem;
    color: var(--bulma-text-weak);
  }

  .transactions-count {
    padding: 0.75rem 1.25rem;
    font-size: 0.8rem;
    color: var(--bulma-text-weak);
    background: var(--bulma-scheme-main-bis);
    border-bottom: 1px solid var(--bulma-border-weak);
  }

  .transactions-list {
    padding: 0.5rem 0;
  }

  .transaction-item {
    padding: 0.875rem 1.25rem;
    border-bottom: 1px solid var(--bulma-border-weak);
  }

  .transaction-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.25rem;
  }

  .transaction-date {
    font-size: 0.75rem;
    color: var(--bulma-text-weak);

    i {
      margin-right: 0.35rem;
    }
  }

  .transaction-amount {
    font-weight: 600;
    color: var(--bulma-success);

    &.negative {
      color: var(--bulma-danger);
    }
  }

  .transaction-payee {
    font-weight: 500;
    color: var(--bulma-text-strong);
    margin-bottom: 0.15rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .transaction-account {
    font-size: 0.8rem;
    color: var(--bulma-text-weak);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
