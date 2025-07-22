<script>
  import { onMount, tick } from 'svelte';
  import * as d3 from 'd3';

  let datasets = {};

  async function fetchData() {
    const res = await fetch('/api/data', {headers: {'Authorization': localStorage.getItem('token') || ''}});
    datasets = await res.json();
    await tick();
    drawAll();
  }

  function safeId(name) {
    return name.replace(/[^A-Za-z0-9_-]/g, '_');
  }

  function drawAll() {
    const margin = {left: 40, right: 20, top: 0, bottom: 20};
    const width = 600 - margin.left - margin.right;
    const height = 300 - margin.top - margin.bottom;
    const scale = 0.01; // sensor values are 100x larger than the displayed units

    for (const [name, info] of Object.entries(datasets)) {
      const data = info.data || [];
      const units = info.units || '';
      const type = info.type || '';
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
      const minVal = d3.min(data, d => d.value * scale);
      const maxVal = d3.max(data, d => d.value * scale);
      const padding = (maxVal - minVal) * 0.1;
      y.domain([minVal - padding, maxVal + padding]);
      const line = d3.line()
        .x(d => x(d.date))
        .y(d => y(d.value * scale));
      g.append('path').datum(data).attr('fill', 'none').attr('stroke', 'steelblue').attr('d', line);
      g.append('g').attr('transform', `translate(0,${height})`).call(d3.axisBottom(x));
      g.append('g').call(d3.axisLeft(y).tickFormat(d3.format('.2f')));
      let label = name;
      const details = [];
      if (type) details.push(type);
      if (units) details.push(units);
      if (details.length) label += ` (${details.join(' ')})`;
      g.append('text')
        .attr('transform', 'rotate(-90)')
        .attr('y', -margin.left + 15)
        .attr('x', -height / 2)
        .attr('text-anchor', 'middle')
        .text(label);
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
