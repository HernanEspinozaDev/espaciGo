export async function actionWithButtonState<T>(
  buttons:HTMLButtonElement[],
  work:()=>Promise<T>,
  refresh:()=>void
):Promise<T>{
  const wasDisabled=buttons.map(button=>button.disabled);
  buttons.forEach(button=>button.disabled=true);
  try{return await work();}
  finally{
    buttons.forEach((button,index)=>button.disabled=wasDisabled[index]);
    refresh();
  }
}
