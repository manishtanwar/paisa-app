<script lang="ts">
  import { onMount } from "svelte";
  import { ajax } from "$lib/utils";

  let status: string = "";
  let tasks: any[] = [];
  let loading = false;
  let error = "";
  let success = "";

  // Format date to "12th July 7:30 PM" format
  function formatDate(dateString: string): string {
    if (!dateString) return "Never";

    const date = new Date(dateString);
    const now = new Date();
    const diffInMinutes = (date.getTime() - now.getTime()) / (1000 * 60);

    // If it's a future date (next run), show relative time
    if (diffInMinutes > 0) {
      if (diffInMinutes < 60) {
        return `in ${Math.floor(diffInMinutes)} minute${
          Math.floor(diffInMinutes) !== 1 ? "s" : ""
        }`;
      }
      const hours = Math.floor(diffInMinutes / 60);
      if (hours < 24) {
        return `in ${hours} hour${hours !== 1 ? "s" : ""}`;
      }
      const days = Math.floor(hours / 24);
      return `in ${days} day${days !== 1 ? "s" : ""}`;
    }

    // If it's a past date, show relative time for recent dates
    const absDiffInHours = Math.abs(diffInMinutes) / 60;
    if (absDiffInHours < 24) {
      if (absDiffInHours < 1) {
        const minutes = Math.floor(Math.abs(diffInMinutes));
        return `${minutes} minute${minutes !== 1 ? "s" : ""} ago`;
      }
      return `${Math.floor(absDiffInHours)} hour${Math.floor(absDiffInHours) !== 1 ? "s" : ""} ago`;
    }

    // Otherwise show formatted date
    const day = date.getDate();
    const suffix = getDaySuffix(day);
    const month = date.toLocaleDateString("en-US", { month: "long" });
    const time = date.toLocaleTimeString("en-US", {
      hour: "numeric",
      minute: "2-digit",
      hour12: true
    });

    return `${day}${suffix} ${month} ${time}`;
  }

  function getDaySuffix(day: number): string {
    if (day >= 11 && day <= 13) return "th";
    switch (day % 10) {
      case 1:
        return "st";
      case 2:
        return "nd";
      case 3:
        return "rd";
      default:
        return "th";
    }
  }

  // Map task names to their API endpoint names
  function getTaskEndpointName(taskName: string): string {
    const taskMap: Record<string, string> = {
      "Daily Trades Fetch": "kite-trades",
      "Daily Price Update": "price-update"
    };
    return taskMap[taskName] || taskName.toLowerCase().replace(/\s+/g, "-");
  }

  async function fetchTasks() {
    loading = true;
    error = "";
    success = "";
    try {
      const res = await ajax("/api/background/tasks");
      status = res.status;
      tasks = res.tasks || [];
    } catch (e) {
      error = "Failed to fetch background tasks.";
    } finally {
      loading = false;
    }
  }

  async function runTask(taskName: string) {
    loading = true;
    error = "";
    success = "";
    try {
      const endpointName = getTaskEndpointName(taskName);
      const res = await ajax(`/api/background/tasks/${endpointName}/run`, { method: "POST" });
      if (res.success) {
        success = res.message || "Task triggered successfully!";
        // Refresh data after running task
        await fetchTasks();
      } else {
        error = res.message || "Failed to trigger task.";
      }
    } catch (e) {
      error = `Failed to trigger ${taskName} task.`;
    } finally {
      loading = false;
    }
  }

  onMount(async () => {
    await fetchTasks();
  });
</script>

<div class="max-w-6xl mx-auto p-6">
  <div class="mb-8">
    <h1 class="text-3xl font-bold text-gray-900 mb-2">Background Tasks</h1>
    <p class="text-gray-600">Monitor and manage automated background tasks</p>
  </div>

  {#if loading}
    <div class="flex items-center justify-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      <span class="ml-3 text-gray-600">Loading...</span>
    </div>
  {/if}

  {#if error}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-6">
      <div class="flex">
        <div class="flex-shrink-0">
          <svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
              clip-rule="evenodd"
            />
          </svg>
        </div>
        <div class="ml-3">
          <p class="text-sm text-red-800">{error}</p>
        </div>
      </div>
    </div>
  {/if}

  {#if success}
    <div class="bg-green-50 border border-green-200 rounded-lg p-4 mb-6">
      <div class="flex">
        <div class="flex-shrink-0">
          <svg class="h-5 w-5 text-green-400" viewBox="0 0 20 20" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
              clip-rule="evenodd"
            />
          </svg>
        </div>
        <div class="ml-3">
          <p class="text-sm text-green-800">{success}</p>
        </div>
      </div>
    </div>
  {/if}

  <!-- Status Card -->
  <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 mb-8">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-lg font-medium text-gray-900">Scheduler Status</h3>
        <p class="text-sm text-gray-500">Background task scheduler status</p>
      </div>
      <div class="flex items-center">
        <div class="flex items-center">
          <div
            class="w-3 h-3 rounded-full {status === 'running' ? 'bg-green-400' : 'bg-red-400'} mr-2"
          ></div>
          <span class="text-sm font-medium capitalize">{status}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Tasks Table -->
  <div class="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
    <div class="px-6 py-4 border-b border-gray-200">
      <h3 class="text-lg font-medium text-gray-900">Background Tasks</h3>
    </div>

    <div class="overflow-x-auto">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th
              class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
            >
              Task Name
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
            >
              Last Run
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
            >
              Last Successful Run
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
            >
              Next Run
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
            >
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          {#each tasks as task}
            <tr class="hover:bg-gray-50">
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center">
                  <div class="flex-shrink-0 h-8 w-8">
                    <div class="h-8 w-8 rounded-full bg-blue-100 flex items-center justify-center">
                      <svg
                        class="h-4 w-4 text-blue-600"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                        />
                      </svg>
                    </div>
                  </div>
                  <div class="ml-4">
                    <div class="text-sm font-medium text-gray-900">{task.task_name}</div>
                    <div class="text-sm text-gray-500">Automated task</div>
                  </div>
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="text-sm text-gray-900">
                  {formatDate(task.last_run)}
                </div>
                {#if task.last_run && !task.success}
                  <div class="text-xs text-red-600">Failed</div>
                {/if}
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="text-sm text-gray-900">
                  {formatDate(task.last_successful_run)}
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="text-sm text-gray-900">
                  {formatDate(task.next_run)}
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                <button
                  class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                  on:click={() => runTask(task.task_name)}
                  disabled={loading}
                >
                  <svg class="h-4 w-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1m4 0h1m-6 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  Run Now
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>

  <!-- Refresh Button -->
  <div class="mt-6 flex justify-end">
    <button
      class="inline-flex items-center px-4 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
      on:click={fetchTasks}
      disabled={loading}
    >
      <svg class="h-4 w-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
        />
      </svg>
      Refresh
    </button>
  </div>
</div>
