<script>
  import { onMount } from 'svelte';
  import * as d3 from 'd3';

  let datasets = {};

  async function fetchData() {
    const res = await fetch('/api/data', {headers: {'Authorization': localStorage.getItem('token') || ''}});
    datasets = await res.json();
    drawAll();
  }

  function safeId(name) {
    return name.replace(/[^A-Za-z0-9_-]/g, '_');
  }

  function drawAll() {
    const margin = {left: 40, right: 20, top: 0, bottom: 20};
    const width = 600 - margin.left - margin.right;
    const height = 300 - margin.top - margin.bottom;

    for (const [name, data] of Object.entries(datasets)) {
      let container = d3.select(`#chart-${safeId(name)}`);
      let svg = container.select('svg');
      let g;
      if (svg.empty()) {
        svg = container.append('svg').attr('width', 600).attr('height', 300);
        g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);
      } else {
        g = svg.select('g');
      }
      g.selectAll('*').remove();
      if (!data.length) continue;
      const x = d3.scaleTime().range([0, width]);
      const y = d3.scaleLinear().range([height, 0]);
      data.forEach(d => { d.date = new Date(d.timestamp * 1000); });
      x.domain(d3.extent(data, d => d.date));
      y.domain([0, d3.max(data, d => d.value)]);
      const line = d3.line()
        .x(d => x(d.date))
        .y(d => y(d.value));
      g.append('path').datum(data).attr('fill', 'none').attr('stroke', 'steelblue').attr('d', line);
      g.append('g').attr('transform', `translate(0,${height})`).call(d3.axisBottom(x));
      g.append('g').call(d3.axisLeft(y));
      g.append('text')
        .attr('transform', 'rotate(-90)')
        .attr('y', 15)
        .attr('x', -140)
        .attr('text-anchor', 'middle')
        .text('Temperature');
    }
  }

  onMount(fetchData);
</script>

<div id="dashboard">
  {#each Object.keys(datasets) as name}
    <div class="chart-container">
      <h3>{name}</h3>
      <div id={"chart-" + safeId(name)}></div>
    </div>
  {/each}
</div>
<button on:click={fetchData}>Refresh</button>

<style>
  #dashboard {
    display: flex;
    flex-wrap: wrap;
  }
  .chart-container {
    margin-right: 20px;
    margin-bottom: 20px;
  }
</style>
