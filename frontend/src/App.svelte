<script>
  import { onMount } from 'svelte';
  import * as d3 from 'd3';

  let datasets = {};
  let selected = '';

  async function fetchData() {
    const res = await fetch('/api/data', {headers: {'Authorization': localStorage.getItem('token') || ''}});
    datasets = await res.json();
    if (!selected && Object.keys(datasets).length) {
      selected = Object.keys(datasets)[0];
    }
    draw();
  }

  let svg;
  function draw() {
    if (!svg) {
      svg = d3.select('#chart').append('svg').attr('width', 600).attr('height', 300);
    }
    svg.selectAll('*').remove();
    const data = datasets[selected] || [];
    if (!data.length) return;
    const x = d3.scaleTime().range([0, 580]);
    const y = d3.scaleLinear().range([280, 0]);
    data.forEach(d => { d.date = new Date(d.timestamp * 1000); });
    x.domain(d3.extent(data, d => d.date));
    y.domain([0, d3.max(data, d => d.value)]);
    const line = d3.line()
      .x(d => x(d.date))
      .y(d => y(d.value));
    svg.append('path').datum(data).attr('fill', 'none').attr('stroke', 'steelblue').attr('d', line);
    svg.append('g').attr('transform', 'translate(0,280)').call(d3.axisBottom(x));
    svg.append('g').call(d3.axisLeft(y));
    svg.append('text')
      .attr('transform', 'rotate(-90)')
      .attr('y', 15)
      .attr('x', -140)
      .attr('text-anchor', 'middle')
      .text('Temperature');
  }

  onMount(fetchData);
</script>

<div id="chart"></div>

<select bind:value={selected} on:change={draw}>
  {#each Object.keys(datasets) as name}
    <option value={name}>{name}</option>
  {/each}
</select>
<button on:click={fetchData}>Refresh</button>
