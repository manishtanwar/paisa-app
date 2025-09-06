<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { generateColorScheme } from "$lib/colors";

  export let account: string = "";
  export let colorScheme: d3.ScaleOrdinal<string, string> | null = null;

  const dispatch = createEventDispatcher();

  interface BreadcrumbSegment {
    label: string;
    fullPath: string;
    isLast: boolean;
  }

  $: segments = generateSegments(account);

  function generateSegments(accountPath: string): BreadcrumbSegment[] {
    if (!accountPath) return [];

    const parts = accountPath.split(":");
    return parts.map((part, index) => ({
      label: part,
      fullPath: parts.slice(0, index + 1).join(":"),
      isLast: index === parts.length - 1
    }));
  }

  function getColor(fullPath: string): string {
    if (colorScheme) {
      return colorScheme(fullPath);
    }
    return "var(--bulma-primary)";
  }

  function handleSegmentClick(segment: BreadcrumbSegment) {
    if (!segment.isLast) {
      dispatch("navigate", { account: segment.fullPath });
    }
  }

  function handleHomeClick() {
    dispatch("navigateHome");
  }
</script>

<nav class="hierarchical-breadcrumb" aria-label="breadcrumbs">
  <ol class="breadcrumb-list">
    <li class="breadcrumb-item">
      <button
        class="breadcrumb-link home-link"
        on:click={handleHomeClick}
        title="Show all categories"
      >
        <span class="icon is-small">
          <i class="fas fa-home"></i>
        </span>
        <span>All</span>
      </button>
    </li>

    {#each segments as segment, index}
      <li class="breadcrumb-separator">
        <span class="icon is-small">
          <i class="fas fa-chevron-right"></i>
        </span>
      </li>

      <li class="breadcrumb-item" class:is-active={segment.isLast}>
        {#if segment.isLast}
          <span
            class="breadcrumb-chip active"
            style="background-color: {getColor(segment.fullPath)}; color: white;"
            title={segment.fullPath}
          >
            {segment.label}
          </span>
        {:else}
          <button
            class="breadcrumb-chip clickable"
            style="background-color: {getColor(segment.fullPath)}20; border-color: {getColor(
              segment.fullPath
            )};"
            on:click={() => handleSegmentClick(segment)}
            title="Navigate to {segment.fullPath}"
          >
            {segment.label}
          </button>
        {/if}
      </li>
    {/each}
  </ol>
</nav>

<style lang="scss">
  .hierarchical-breadcrumb {
    padding: 0.5rem 0;
  }

  .breadcrumb-list {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.25rem;
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .breadcrumb-item {
    display: flex;
    align-items: center;
  }

  .breadcrumb-separator {
    display: flex;
    align-items: center;
    color: var(--bulma-text-weak);
    font-size: 0.7rem;
  }

  .breadcrumb-link {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.35rem 0.6rem;
    background: var(--bulma-scheme-main-bis);
    border: 1px solid var(--bulma-border);
    border-radius: 4px;
    color: var(--bulma-text);
    font-size: 0.8rem;
    cursor: pointer;
    transition: all 0.15s ease;

    &:hover {
      background: var(--bulma-primary);
      color: var(--bulma-primary-invert);
      border-color: var(--bulma-primary);
    }

    &.home-link {
      .icon {
        margin-right: 0.15rem;
      }
    }
  }

  .breadcrumb-chip {
    padding: 0.35rem 0.75rem;
    border-radius: 16px;
    font-size: 0.8rem;
    font-weight: 500;
    transition: all 0.15s ease;
    white-space: nowrap;

    &.clickable {
      background: none;
      border: 1px solid;
      cursor: pointer;
      color: var(--bulma-text);

      &:hover {
        filter: brightness(0.9);
        transform: translateY(-1px);
      }
    }

    &.active {
      border: none;
      cursor: default;
      box-shadow: 0 2px 4px rgba(0, 0, 0, 0.15);
    }
  }
</style>
