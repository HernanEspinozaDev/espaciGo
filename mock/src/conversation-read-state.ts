export type ReadPage = {items: readonly {sequence:number}[]};

// Mark only after a successful page load has actually been rendered. A null
// load or stale selection is discarded without advancing the participant's
// cursor; an empty thread has no sequence to acknowledge.
export async function showThenMarkConversationPage<T extends ReadPage>(
  load:()=>Promise<T|null>,
  show:(page:T)=>boolean,
  mark:(throughSequence:number)=>Promise<void>
):Promise<number|null>{
  const page=await load();
  if(page===null||!show(page)||page.items.length===0)return null;
  const through=Math.max(...page.items.map(item=>item.sequence));
  await mark(through);
  return through;
}
