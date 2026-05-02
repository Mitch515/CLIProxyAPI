<script lang="ts">
  interface Props { height?: string; width?: string; radius?: string; }
  let { height = '14px', width = '100%', radius = 'var(--r-md)' }: Props = $props();
</script>

<span class="skeleton" style="height: {height}; width: {width}; border-radius: {radius};" aria-hidden="true"></span>

<style>
  .skeleton { display: inline-block; }
</style>
