module View.Pages.Grid exposing (view)

import Html exposing (Html, button, div, h2, input, label, p, span, text)
import Html.Attributes exposing (class, value)
import Html.Events exposing (onClick, onInput)
import Model exposing (Field(..), GridForm, Model, Msg(..), Tab(..))
import View.Components.Form as Form
import View.Components.JobStatus as JobStatus
import View.Components.Preview as Preview


view : Model -> Html Msg
view model =
    let
        g =
            model.grid
    in
    div []
        [ div [ class "card" ]
            [ h2 [] [ text "Power / speed test grid" ]
            , p [ class "muted" ] [ text "One row per power, one column per feed rate. The grid is centred on the work origin." ]
            , h2 [] [ text "Row powers (bottom to top)" ]
            , div [] (List.indexedMap (valueRow "Row" "%" SetGridPower RemoveGridPower) g.powers)
            , Form.button "Add row" True AddGridPower
            , h2 [ class "spaced" ] [ text "Column feeds (left to right)" ]
            , div [] (List.indexedMap (valueRow "Column" "mm/min" SetGridFeed RemoveGridFeed) g.feeds)
            , Form.button "Add column" True AddGridFeed
            , div [ class "fields spaced" ]
                [ Form.field "Cell size (mm)" g.cell (SetField GridCell)
                , Form.field "Gap (mm)" g.gap (SetField GridGap)
                , Form.field "Line interval (mm)" g.interval (SetField GridInterval)
                ]
            , Form.framingSelect model.framing
            , div [ class "row" ] [ Form.button "Burn test grid" (JobStatus.canRun model.machine) (RunJob GridTab) ]
            , JobStatus.view model.machine
            ]
        , div [ class "card" ] [ Preview.view model (annotations g) model.gridPreview ]
        ]


valueRow : String -> String -> (Int -> String -> Msg) -> (Int -> Msg) -> Int -> String -> Html Msg
valueRow name unit set remove i v =
    label [ class "value-row" ]
        [ span [ class "value-name" ] [ text (name ++ " " ++ String.fromInt (i + 1)) ]
        , input [ value v, onInput (set i) ] []
        , span [ class "unit" ] [ text unit ]
        , button [ class "btn btn-small", onClick (remove i) ] [ text "✕" ]
        ]


{-| Labels for the preview: feeds under each column, powers left of each
row, laid out the same way as the server's grid.
-}
annotations : GridForm -> List Preview.Annotation
annotations g =
    case ( String.toFloat g.cell, String.toFloat g.gap ) of
        ( Just cell, Just gap ) ->
            let
                pitch =
                    cell + gap

                cols =
                    List.length g.feeds

                rows =
                    List.length g.powers

                width =
                    toFloat cols * pitch - gap

                height =
                    toFloat rows * pitch - gap

                x0 =
                    -width / 2

                y0 =
                    -height / 2

                feedLabel i f =
                    { x = x0 + toFloat i * pitch + cell / 2, y = y0 - 1.5, text = f, anchor = "middle" }

                powerLabel i pw =
                    { x = x0 - 1.5, y = y0 + toFloat i * pitch + cell / 2, text = pw ++ "%", anchor = "end" }
            in
            List.indexedMap feedLabel g.feeds ++ List.indexedMap powerLabel g.powers

        _ ->
            []
