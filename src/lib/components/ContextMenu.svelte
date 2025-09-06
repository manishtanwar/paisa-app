<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import { fade } from "svelte/transition";

  export let x: number;
  export let y: number;
  export let account: string;
  export let hasChildren: boolean = true;

  const dispatch = createEventDispatcher();

  let menuRef: HTMLDivElement;

  onMount(() => {
    // Adjust position if menu would go off screen
    const rect = menuRef.getBoundingClientRect();
    if (rect.right > window.innerWidth) {
      x = window.innerWidth - rect.width - 10;
    }
    if (rect.bottom > window.innerHeight) {
      y = window.innerHeight - rect.height - 10;
    }

    // Add click outside listener
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef && !menuRef.contains(e.target as Node)) {
        dispatch("close");
      }
    };

    // Add escape key listener
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        dispatch("close");
      }
    };

    document.addEventListener("click", handleClickOutside, true);
    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.removeEventListener("click", handleClickOutside, true);
      document.removeEventListener("keydown", handleKeyDown);
    };
  });

  function handleZoom() {
    dispatch("zoom", { account });
    dispatch("close");
  }

  function handleViewTransactions() {
    dispatch("viewTransactions", { account });
    dispatch("close");
  }

  function handleViewTrend() {
    dispatch("viewTrend", { account });
    dispatch("close");
  }
</script>

<div
  bind:this={menuRef}
  class="context-menu"
  style="left: {x}px; top: {y}px;"
  transition:fade={{ duration: 150 }}
>
  <div class="menu-header">
    <span class="icon is-small">
      <i class="fas fa-folder"></i>
    </span>
    <span class="account-name" title={account}>{account.split(":").pop()}</span>
  </div>

  <hr class="menu-divider" />

  {#if hasChildren}
    <button class="menu-item" on:click={handleZoom}>
      <span class="icon">
        <i class="fas fa-search-plus"></i>
      </span>
      <span>Zoom Into Subaccounts</span>
    </button>
  {/if}

  <button class="menu-item" on:click={handleViewTransactions}>
    <span class="icon">
      <i class="fas fa-list"></i>
    </span>
    <span>View All Transactions</span>
  </button>

  <button class="menu-item" on:click={handleViewTrend}>
    <span class="icon">
      <i class="fas fa-chart-line"></i>
    </span>
    <span>View Trend Analysis</span>
  </button>
</div>

<style lang="scss">
  .context-menu {
    position: fixed;
    z-index: 1000;
    min-width: 220px;
    background: #ffffff;
    border-radius: 8px;
    box-shadow:
      0 8px 24px rgba(0, 0, 0, 0.2),
      0 2px 8px rgba(0, 0, 0, 0.1);
    padding: 0.5rem 0;
    border: 1px solid #e0e0e0;
    overflow: hidden;
  }

  :global(.dark) .context-menu {
    background: #1a1a2e;
    border-color: #3a3a5a;
  }

  .menu-header {
    padding: 0.75rem 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--bulma-text-strong);
    font-weight: 600;
    font-size: 0.875rem;
  }

  .account-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 180px;
  }

  .menu-divider {
    margin: 0.25rem 0;
    border: none;
    border-top: 1px solid var(--bulma-border-weak);
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    padding: 0.6rem 1rem;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    color: var(--bulma-text);
    font-size: 0.875rem;
    transition: background-color 0.15s ease;

    &:hover {
      background-color: var(--bulma-primary);
      color: var(--bulma-primary-invert);

      .icon {
        color: var(--bulma-primary-invert);
      }
    }

    .icon {
      width: 1.25rem;
      text-align: center;
      color: var(--bulma-text-weak);
      transition: color 0.15s ease;
    }
  }
</style>
