module View.Components.Form exposing (button, checkbox, dangerButton, field, framingSelect, select)

import Html exposing (Html, div, input, label, option, span, text)
import Html.Attributes exposing (checked, class, disabled, selected, type_, value)
import Html.Events exposing (onCheck, onClick, onInput)
import Model exposing (Msg(..))


field : String -> String -> (String -> Msg) -> Html Msg
field name v toMsg =
    label [ class "field" ]
        [ span [ class "field-label" ] [ text name ]
        , input [ value v, onInput toMsg ] []
        ]


select : String -> String -> List ( String, String ) -> (String -> Msg) -> Html Msg
select name current options toMsg =
    label [ class "field" ]
        [ span [ class "field-label" ] [ text name ]
        , Html.select [ onInput toMsg, value current ]
            (List.map
                (\( v, t ) -> option [ value v, selected (v == current) ] [ text t ])
                options
            )
        ]


checkbox : String -> Bool -> Msg -> Html Msg
checkbox name v msg =
    label [ class "checkbox" ]
        [ input [ type_ "checkbox", checked v, onCheck (\_ -> msg) ] []
        , text name
        ]


button : String -> Bool -> Msg -> Html Msg
button name enabled msg =
    Html.button [ class "btn", disabled (not enabled), onClick msg ] [ text name ]


dangerButton : String -> Msg -> Html Msg
dangerButton name msg =
    Html.button [ class "btn btn-danger", onClick msg ] [ text name ]


framingSelect : String -> Html Msg
framingSelect current =
    div []
        [ select "Start"
            current
            [ ( "full", "Enter laser mode (M321, tool change to laser)" )
            , ( "resume", "Laser already loaded (M321.2)" )
            ]
            SetFraming
        ]
