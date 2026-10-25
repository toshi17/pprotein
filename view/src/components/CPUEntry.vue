<template>
  <section>
    <pre>{{ $data.text || $data.summary }}</pre>
  </section>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
  data() {
    return {
      summary: "Loading ...",
      text: "",
    };
  },
  async created() {
    await this.updateText(this.$route.params.id);
  },
  async beforeRouteUpdate(route) {
    await this.updateText(route.params.id);
  },
  methods: {
    async updateText(id: string | string[]) {
      try {
        const resp = await fetch(`/api/cpu/data/${id}`);
        if (!resp.ok) {
          throw new Error(`${resp.status} ${resp.statusText}`);
        }
        this.text = await resp.text();
      } catch (e) {
        this.summary = `Error: ${e instanceof Error ? e.message : e}`;
      }
    },
  },
});
</script>
