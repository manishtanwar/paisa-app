<script lang="ts">
  import type { Directory, LedgerFile } from "$lib/utils";
  import _ from "lodash";
  import { createEventDispatcher } from "svelte";

  export let files: Array<Directory | LedgerFile>;
  export let path: string;
  export let selectedFileName: string;
  export let hasUnsavedChanges: boolean;
  export let root: boolean = true;

  const dispatch = createEventDispatcher();

  function fileName(path: string) {
    return _.last(path.split("/"));
  }

  function join(paths: string[]) {
    return _.filter(paths, (p) => !_.isEmpty(p)).join("/");
  }

  function isOpen(file: Directory | LedgerFile) {
    const fullPath = join([path, file.name]);
    return selectedFileName?.startsWith(fullPath);
  }

  function handleCreateInDir(e: Event, dirPath: string) {
    e.stopPropagation();
    dispatch("createInDir", dirPath);
  }
</script>

<ul class={root && "du-menu du-menu-sm w-full p-0"}>
  {#each files as file}
    {#if file.type != "directory"}
      <li>
        <a
          on:click={() => dispatch("select", file)}
          class={file.name == selectedFileName ? "du-active" : ""}
        >
          <span class="icon is-small">
            <i class="fa-regular fa-file-lines" />
          </span>
          <span title={fileName(file.name)} class="truncate">{fileName(file.name)}</span>
          {#if file.name == selectedFileName && hasUnsavedChanges}
            <span class="ml-1 tag is-danger">unsaved</span>
          {/if}
        </a>
      </li>
    {:else}
      <li>
        <details open={isOpen(file)}>
          <summary class="is-flex is-align-items-center">
            <span class="icon is-small">
              <i class="fa-regular fa-folder" />
            </span>
            <span title={file.name} class="truncate is-flex-grow-1">{file.name}</span>
            <button
              class="button is-small is-white is-rounded ml-1 create-in-dir-btn"
              title="Create file in {file.name}"
              on:click={(e) => handleCreateInDir(e, join([path, file.name]))}
            >
              <span class="icon is-small">
                <i class="fas fa-plus" />
              </span>
            </button>
          </summary>
          <svelte:self
            path={join([path, file.name])}
            on:select={(e) => dispatch("select", e.detail)}
            on:createInDir={(e) => dispatch("createInDir", e.detail)}
            root={false}
            files={file.children}
            {selectedFileName}
            {hasUnsavedChanges}
          />
        </details>
      </li>
    {/if}
  {/each}
</ul>

<style>
  .create-in-dir-btn {
    opacity: 0;
    transition: opacity 0.2s ease;
    padding: 0;
    height: 1.5em;
    width: 1.5em;
    min-width: 1.5em;
  }

  summary:hover .create-in-dir-btn {
    opacity: 1;
  }
</style>
