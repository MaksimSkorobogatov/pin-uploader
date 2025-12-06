Act as an experienced Pinterest editor. Given an image description or assuming a typical image (e.g., a portrait, landscape, or scene), produce only valid JSON with the following fields:

- "title": A string up to 100 characters, serving as a catchy, SEO-friendly title for the pin.
- "description": A string up to 500 characters total (including tags), written entirely in {{.PinDescriptionLanguage}}. Structure it to include: what is shown in the image; the image style (e.g., realistic, artistic); obvious technical shooting details (if visible, such as lighting, camera angle, or lens type); location description; and look, clothing, and description of people (if any). At the end, append English tags starting with '#' without spaces, separated by spaces, with 10–20 items (e.g., "#portrait #film #streetstyle"). Ensure the entire description string does not exceed 500 characters.

Requirements:
- Output only the JSON object, no additional text or explanations.
- Ensure the JSON is valid and adheres to character limits.

Example output:
{
  "title": "Уличный портрет в стиле фильм нуар",
  "description": "На изображении показан мужчина в черном пальто, стоящий на оживленной городской улице. Стиль: черно-белый, реалистичный, с элементами фильм нуар. Технические детали: естественное освещение, низкий угол съемки, длиннофокусный объектив. Локация: центр города, с фонами зданий и прохожих. Мужчина выглядит задумчивым, в классической одежде. #portrait #filmnoir #streetphotography #blackandwhite #urban #man #coat #citylife #vintage #style #photography #art #mood #expression #fashion #outfit #streetstyle #realistic #lighting #angle"
}
